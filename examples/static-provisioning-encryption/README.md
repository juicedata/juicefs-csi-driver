# Static Provisioning With Encryption

This example shows how to make a static provisioned JuiceFS persistence volume (PV) mounted with encryption method inside container.

## Provide secret information

First we create an RSA key:

```sh
openssl genrsa -out aaron.pem -aes256 2048
```

and the passphrase is `testpass`. It will be referred in `kustomization.yaml`

## Apply patches to CSI driver and PV

We need to provide RSA key file and passphrase environment variables to node driver plugin. Include [driver/patches.yaml](driver/patches.yaml) as an overlay.

Patch the persistent volume with `mountOptions` as in [app/patches.yaml](app/patches.yaml).

Note that the two bases must be separated if they are in different namespaces due to the limitation of `kustomize`

Apply the patched manifests.

```sh
kustomize build | kubectl apply -f -
```

## Check JuiceFS filesystem is used

After the objects are created, verify that pod is running:

```sh
>> kubectl get pods
```

Also you can verify that data is written onto JuiceFS filesystem:

```sh
>> kubectl exec -ti app -- tail -f /data/out.txt
```
