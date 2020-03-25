# Static Provisioning With Encryption

This example shows how to make a static provisioned JuiceFS persistence volume (PV) mounted with encryption method inside container.


## Provide secret information

First we create an RSA key: 

```
openssl genrsa -out aaron.pem -aes256 2048
```

and the passphrase is `testpass`, will be used later.

Using `kubectl apply -k .` to create secrets containing the content of RSA key and RSA key passphrase. Or create it by your way.

## Reinstall the CSI-Plugin

We need to provide RSA key file and passphrase environment variables in CSI-Node containers. So use `csi.yaml` to reinstall CSI-Plugin:

```
kubectl apply -f csi.yaml
```

## Apply the configurations

```
kubectl apply -f k8s.yaml
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
