# Mount config and envs

This example shows how to mount config files or set envs in JuiceFS mount pod.

## Provide secret information

The CSI driver support to mount config files or set envs in JuiceFS mount pod.

This example uses google cloud platform as object. Please follow Google Cloud document to know
how [authentication](https://cloud.google.com/docs/authentication)
and [authorization](https://cloud.google.com/iam/docs/overview) work. And you create gc credential config in a right way.

Put the result of base64 gc credential config in a Kubernetes secret as file `gc-secret.yaml`, and the key is the config file you will put in
mount pod:

```yaml
apiVersion: v1
data:
  application_default_credentials.json: eyAiY2xpZW50X2lkIjogIjc2NDA4NjA1MTg1MC02cXI0cDZncGk2aG41MDZwdDhlanVxODNkaT*****=
kind: Secret
metadata:
  name: gc-secret
  namespace: kube-system
type: Opaque
```

Then you need to provide a secret file `juicefs-secrets.env` containing the required credentials. This secret is for JuiceFS CSI driver, and you need put configs and envs as json as bellow. 
The key of `configs` is the secret name, value is the path of secret being mounted in pod.

```
bucket=<bucket>
envs={GOOGLE_CLOUD_PROJECT: "/root/.config/gcloud/application_default_credentials.json"}
configs={"gc-secret": "/root/.config/gcloud"}
metaurl=<metaurl>
name=<name>
storage=<storage>
```

## Apply the configurations

Build the example with [kustomize](https://github.com/kubernetes-sigs/kustomize) and apply with `kubectl`:

```sh
kustomize build | kubectl apply -f -
```

Or apply with `kubectl` >= 1.14:

```sh
kubectl apply -k .
```

## Check JuiceFS file system is used

After the objects are created, verify that pod is running:

```sh
kubectl get pods juicefs-app
```

Verify that env you set:

```sh
$ kubectl get po juicefs-kube-node-3-test-bucket -oyaml |grep env -A 4
    env:
    - name: JFS_FOREGROUND
      value: "1"
    - name: GOOGLE_CLOUD_PROJECT
      value: /root/.config/gcloud/application_default_credentials.json
```

Also you can verify that gc credential config is in path you set:

```sh
$ kubectl exec -it juicefs-kube-node-3-test-bucket -- cat /root/.config/gcloud/application_default_credentials.json
{ "client_id": "764086051850-6qr4p6g****", "client_secret": "*****", "refresh_token": "******", "type": "authorized_user" }
```
