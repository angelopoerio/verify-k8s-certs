FROM alpine 
ADD ./verify-k8s-certs /verify-k8s-certs
CMD ["chmod", "+x", "/verify-k8s-certs"]
ENTRYPOINT ["/verify-k8s-certs"]
