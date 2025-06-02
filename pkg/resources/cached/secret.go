package cached

// used as a CR sample, hard coded for editing
var SecretCr = `apiVersion: v1
kind: Secret
metadata:
  name: dotfile-secret
data:
  .secret-file: dmFsdWUtMg0KDQo=
`

// fallback crd when k8s is not available
var SecretSchema = `=== Cached ===
KIND:       Secret
VERSION:    v1

DESCRIPTION:
    Secret holds secret data of a certain type. The total bytes of the values in
    the Data field must be less than MaxSecretSize bytes.
    
FIELDS:
  apiVersion	<string>
  data	<map[string]string>
  immutable	<boolean>
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
  stringData	<map[string]string>
  type	<string>

`
