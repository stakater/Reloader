# Using Reloader with Sealed Secrets

Below are the steps to use Reloader with Sealed Secrets:

1. Download and install the kubeseal client from [here](https://github.com/bitnami-labs/sealed-secrets)
1. Install the controller for Sealed Secrets
1. Fetch the encryption certificate
1. Encrypt the secret
1. Apply the secret
1. Install the tool which uses that Sealed Secret
1. Install Reloader
1. Once everything is setup, update the original secret at client and encrypt it with kubeseal to see Reloader working
1. Apply the updated Sealed Secret
1. Reloader will restart the pod to use that updated secret
