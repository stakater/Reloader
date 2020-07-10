Below are the steps to use reloader with Sealed Secrets.
1. Download and install the kubeseal client from [here](https://github.com/bitnami-labs/sealed-secrets).
2. Install the controller for sealed secrets
3. Fetch the encryption certificate
4. Encrypt the secret.
5. Apply the secret.
7. Install the tool which uses that sealed secret.
8. Install Reloader.
9. Once everything is setup, update the original secret at client and encrypt it with kubeseal to see reloader working.
10. Apply the updated sealed secret.
11. Reloader will restart the pod to use that updated secret.
