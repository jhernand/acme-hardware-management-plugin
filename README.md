# O-RAN O2 IMS hardware management plugin example

This project is an example of how to write a hardware management plugin for Red
Hat's O-RAN O2 IMS implementation.

## Building

To build the project make sure that you have a working version of Go 1.22 and
then simply type `go build` in the top level directory of the project. It will
generate a `acme-hardware-management-plugin` binary.

## Running

The plugin is a Kubernetes controller that watches objects of kind
`NodeAllocationRequest` and `NodeReleaseRequest` and acts accordingly.

To play with it you will need any Kubernetes cluster. Then you will need to
apply the custom resource definitions for the `NodeAllocationRequest` and
`NodeReleaseRequest`. They are available in in the
[openshift-knk/oran-o2ims](https://github.com/openshift-kni/oran-o2ims)
project, inside the
[config/cdr/bases](https://github.com/openshift-kni/oran-o2ims/tree/main/config/crd/bases)
directory, so you can create them like this:

```shell
$ kubectl create -f https://raw.githubusercontent.com/openshift-kni/oran-o2ims/main/config/crd/bases/hardwaremanagement.oran.openshift.io_nodeallocationrequests.yaml
$ kubectl create -f https://raw.githubusercontent.com/openshift-kni/oran-o2ims/main/config/crd/bases/hardwaremanagement.oran.openshift.io_nodereleaserequests.yaml
```

Then start the controller in one window:

```shell
$ ./acme-hardware-management-plugin
```

This will run in the foreground and will write log messages to the output.

To simulate the O-RAN O2 IMS you can, in another window, manually create a
node allocation request:

```shell
$ kubectl create -f - <<.
kind: NodeAllocationRequest
apiVersion: hardwaremanagement.oran.openshift.io/v1alpha1
metadata:
  namespace: europe
  name: cu-113d00a-22422

spec:
  cloudID: "0f34b4cf-41a0-4b49-bcb3-9f2daebfbee7"
  location: madrid
  extensions:
    "oran.acme.com/model": "BigIron X42"
    "oran.acme.com/firmwareSettings": |
       {
         "MinProcIdlePower": "C6"
       }
    "oran.acme.com/firmwareVersions": |
       {
         "BigIron UEFI": "4.11",
         "Intel(R) E810-XXVDA2": "2.50"
       }
.
```

Observe in the logs how the controller fulfills the request and check the result, in particular the
`status`, where the controller will write the details of the node, in particular the BMC address and
the name of the secret containing the BMC user name and password:

```shell
$ kubectl get nodeallocationrequest -n europe cu-113d00a-22422 -o yaml
$ kubectl get secret -n europe cu-113d00a-22422-bmc -o yaml
```

You can also create a node deallocation request: 

```shell
$ kubectl create -f - <<.
kind: NodeReleaseRequest
apiVersion: hardwaremanagement.oran.openshift.io/v1alpha1
metadata:
  namespace: europe
  name: cu-113d00a-22422

spec:
  cloudID: "0f34b4cf-41a0-4b49-bcb3-9f2daebfbee7"
  nodeID: "acme-c3f64cf1-40c0-4324-b13a-40f22b0d16c5"
.
```

And check the result:

```shell
$ kubectl get nodereleaserequest -n europe cu-113d00a-22422
```
