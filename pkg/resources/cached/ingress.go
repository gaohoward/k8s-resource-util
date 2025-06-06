package cached

// used as a CR sample, hard coded for editing
var IngressCr = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: example-ingress
spec:
  ingressClassName: nginx
  rules:
    - host: hello-world.example
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web
                port:
                  number: 8080
`

// fallback crd when k8s is not available
var IngressSchema = `GROUP:      networking.k8s.io
KIND:       Ingress
VERSION:    v1

DESCRIPTION:
    Ingress is a collection of rules that allow inbound connections to reach the
    endpoints defined by a backend. An Ingress can be configured to give
    services externally-reachable urls, load balance traffic, terminate SSL,
    offer name based virtual hosting etc.
    
FIELDS:
  apiVersion	<string>
  kind	<string>
  metadata	<ObjectMeta>
    annotations	<map[string]string>
    creationTimestamp	<string>
    deletionGracePeriodSeconds	<integer>
    deletionTimestamp	<string>
    finalizers	<[]string>
    generateName	<string>
    generation	<integer>
    labels	<map[string]string>
    managedFields	<[]ManagedFieldsEntry>
      apiVersion	<string>
      fieldsType	<string>
      fieldsV1	<FieldsV1>
      manager	<string>
      operation	<string>
      subresource	<string>
      time	<string>
    name	<string>
    namespace	<string>
    ownerReferences	<[]OwnerReference>
      apiVersion	<string> -required-
      blockOwnerDeletion	<boolean>
      controller	<boolean>
      kind	<string> -required-
      name	<string> -required-
      uid	<string> -required-
    resourceVersion	<string>
    selfLink	<string>
    uid	<string>
  spec	<IngressSpec>
    defaultBackend	<IngressBackend>
      resource	<TypedLocalObjectReference>
        apiGroup	<string>
        kind	<string> -required-
        name	<string> -required-
      service	<IngressServiceBackend>
        name	<string> -required-
        port	<ServiceBackendPort>
          name	<string>
          number	<integer>
    ingressClassName	<string>
    rules	<[]IngressRule>
      host	<string>
      http	<HTTPIngressRuleValue>
        paths	<[]HTTPIngressPath> -required-
          backend	<IngressBackend> -required-
            resource	<TypedLocalObjectReference>
              apiGroup	<string>
              kind	<string> -required-
              name	<string> -required-
            service	<IngressServiceBackend>
              name	<string> -required-
              port	<ServiceBackendPort>
                name	<string>
                number	<integer>
          path	<string>
          pathType	<string> -required-
          enum: Exact, ImplementationSpecific, Prefix
    tls	<[]IngressTLS>
      hosts	<[]string>
      secretName	<string>
  status	<IngressStatus>
    loadBalancer	<IngressLoadBalancerStatus>
      ingress	<[]IngressLoadBalancerIngress>
        hostname	<string>
        ip	<string>
        ports	<[]IngressPortStatus>
          error	<string>
          port	<integer> -required-
          protocol	<string> -required-
          enum: SCTP, TCP, UDP

`
