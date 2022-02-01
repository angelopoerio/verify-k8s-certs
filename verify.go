package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	log "github.com/sirupsen/logrus"
)

var (
	expiredCertsGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tls_verifier_seconds_to_expiration_tls_certificate",
		Help: "Seconds to expiration for the TLS certificate of the service",
	}, []string{"namespace", "service", "port", "issuer", "serialnumber"})
	discoveredCertsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tls_verifier_discovered_tls_certificates_of_services",
		Help: "How many TLS certificates have been discovered across all the services",
	})
	hearthbeatCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tls_verifier_heartbeat",
		Help: "heartbeat counter that keeps increasing if service is healthy",
	})
)

func testTLS(tlsTimeout time.Duration, svc string, namespace string, port int32) (bool, int) {
	fullhostname := fmt.Sprintf("%s.%s.svc.cluster.local:%d", svc, namespace, port)

	conf := tls.Config{
		InsecureSkipVerify: true,
	}

	dialer := &net.Dialer{
		Timeout: tlsTimeout,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", fullhostname, &conf)
	if err != nil {
		log.Errorf("Could not start a TLS connection to %s: %v\n", fullhostname, err)
		return false, 0
	}

	defer conn.Close()

	_, err = conn.Write([]byte("ping\n"))
	if err != nil {
		log.Errorf("Could not send data to %s: %v\n", fullhostname, err)
		return false, 0
	}

	certs := conn.ConnectionState().PeerCertificates
	certsExpiryDates := make([]string, 10)
	discoveredTLScerts := 0
	for _, cert := range certs {
		discoveredTLScerts++
		certsExpiryDates = append(certsExpiryDates, cert.NotAfter.Format("2006-January-02"))
		timeToExpiration := cert.NotAfter.Sub(time.Now())
		expiredCertsGauge.WithLabelValues(namespace, svc, strconv.Itoa(int(port)), cert.Issuer.CommonName, cert.Issuer.SerialNumber).Set(timeToExpiration.Seconds())
	}

	log.Infof("TLS connection was successful to %s. Certs expiration dates: %v\n", fullhostname, certsExpiryDates)
	return true, discoveredTLScerts
}

func discoverServices(discoverFrequency time.Duration, tlsTimeout time.Duration, skipNamespaceRegex string) int {

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	r, err := regexp.Compile(skipNamespaceRegex)

	if skipNamespaceRegex != "" && err != nil {
		panic(err.Error())
	}

	for {
		discoveredTLScertificates := 0
		services, err := clientset.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}

		log.Infof("Scanning for %d services for expired TLS certificates ...\n", len(services.Items))

		for _, svc := range services.Items {
			ports := svc.Spec.Ports
			ns := svc.GetNamespace()
			svcName := svc.GetName()

			if skipNamespaceRegex != "" && r.Match([]byte(ns)) {
				log.Infof("Skipping service:%s in namespace: %s", svcName, ns)
				continue
			}

			for _, port := range ports {
				if ok, certsNum := testTLS(tlsTimeout, svcName, ns, port.Port); ok {
					discoveredTLScertificates += certsNum
				}
			}

		}

		discoveredCertsGauge.Set(float64(discoveredTLScertificates))
		hearthbeatCounter.Inc()

		log.Infof("Sleeping for %v until the next scan", discoverFrequency)
		time.Sleep(discoverFrequency)
	}
}

func main() {

	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	discoverFrequency := flag.String("frequency", "2h", "How often to scan for new TLS certs")
	tlsTimeout := flag.String("timeout", "400ms", "Connection timeout to TLS endpoints")
	skipNamespaceRegex := flag.String("skip-namespace-regex", "", "Namespaces matching this regex get skipped")
	port := flag.Int("port", 9999, "the tcp port where to listen on")
	flag.Parse()

	discoverFrequencyDuration, err := time.ParseDuration(*discoverFrequency)

	if err != nil {
		fmt.Printf("Invalid specified frequency: %v\n", err)
		os.Exit(1)
	}

	tlsTimeoutDuration, err := time.ParseDuration(*tlsTimeout)

	if err != nil {
		fmt.Printf("Invalid specified TLS timeout: %v\n", err)
		os.Exit(1)
	}

	go discoverServices(discoverFrequencyDuration, tlsTimeoutDuration, *skipNamespaceRegex)

	healthcheckHandler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Mi sento bene!")
	}

	listenAddr := fmt.Sprintf(":%d", *port)
	log.Infof("Listening for metrics and healthchecks on %s", listenAddr)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/livez", healthcheckHandler) /* useful for k8s healthchecks */
	http.HandleFunc("/healthz", healthcheckHandler)
	http.ListenAndServe(listenAddr, nil)
}
