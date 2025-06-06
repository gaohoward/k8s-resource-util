package cached

// used as a CR sample, hard coded for editing
var PvcCr = `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: extra-jars
  namespace: default
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 500Mi
`

// fallback crd when k8s is not available
var PvcSchema = `=== Cached ===
KIND:       PersistentVolumeClaim
VERSION:    v1

DESCRIPTION:
    PersistentVolumeClaim is a user's request for and claim to a persistent
    volume
    
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
  spec	<PersistentVolumeClaimSpec>
    accessModes	<[]string>
    dataSource	<TypedLocalObjectReference>
      apiGroup	<string>
      kind	<string> -required-
      name	<string> -required-
    dataSourceRef	<TypedObjectReference>
      apiGroup	<string>
      kind	<string> -required-
      name	<string> -required-
      namespace	<string>
    resources	<VolumeResourceRequirements>
      limits	<map[string]Quantity>
      requests	<map[string]Quantity>
    selector	<LabelSelector>
      matchExpressions	<[]LabelSelectorRequirement>
        key	<string> -required-
        operator	<string> -required-
        values	<[]string>
      matchLabels	<map[string]string>
    storageClassName	<string>
    volumeAttributesClassName	<string>
    volumeMode	<string>
    enum: Block, Filesystem
    volumeName	<string>
  status	<PersistentVolumeClaimStatus>
    accessModes	<[]string>
    allocatedResourceStatuses	<map[string]string>
    allocatedResources	<map[string]Quantity>
    capacity	<map[string]Quantity>
    conditions	<[]PersistentVolumeClaimCondition>
      lastProbeTime	<string>
      lastTransitionTime	<string>
      message	<string>
      reason	<string>
      status	<string> -required-
      type	<string> -required-
    currentVolumeAttributesClassName	<string>
    modifyVolumeStatus	<ModifyVolumeStatus>
      status	<string> -required-
      enum: InProgress, Infeasible, Pending
      targetVolumeAttributesClassName	<string>
    phase	<string>
    enum: Bound, Lost, Pending

`
