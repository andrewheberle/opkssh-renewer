# opkssh-renewer

This is a simple wrapper for [opkssh](https://github.com/openpubkey/opkssh) to renew certificates and (re-)add them to a running ssh-agent as required.

By default if the expect key/certificate is not found or is older than 23-hours it will be renewed by running "opkssh login".
