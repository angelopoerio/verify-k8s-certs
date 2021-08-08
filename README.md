

# About
verify-k8s-certs is a daemon (prometheus exporter) to discover expired TLS certificates in a kubernetes cluster. It exposes the informations
as Prometheus metrics that can be scraped.


# Build & dockerize
Build the daemon:

```bash
go build -o verify-k8s-certs
```

Build the docker image:

```bash
docker build -t verify-k8s-certs .
```


# How to run
Be sure to run the daemon as a kubernetes **deployment**, you should also expose it as a **service** so Prometheus can
scrape the metrics from its endpoints.
The service needs permission to list all the **namespaces** and all the services of the cluster
so be sure to use a **serviceaccount** with these privileges otherwise it will not work!
When the deployment runs on the cluster with no errors then you should add to to the **scrape_config** section of your Prometheus instance a new job
to instruct it to scrape the metrics.  

# Metrics
The exposed Prometheus metrics are the following ones (at the endpoint **/metrics**):
* (gauge) **tls_verifier_seconds_to_expiration_tls_certificate**: how many seconds are left to the expiration of the certificate for the services
* (gauge) **tls_verifier_discovered_tls_certificates_of_services**: how many TLS certificates have been discovered in the exposed services of the cluster
* (counter) **tls_verifier_heartbeat**: just a counter that keeps increasing, it can be used to detect if the daemon is healthy or not

# Author
Angelo Poerio <angelo.poerio@gmail.com>
