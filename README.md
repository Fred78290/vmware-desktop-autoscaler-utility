# vmware-desktop-autoscaler-utility

This tool is an internal helper for [kubernetes-desktop-autoscaler](https://github.com/Fred78290/kubernetes-desktop-autoscaler).

It wrap vmrest and vmrun vmware tools from vmware desktop and provide functions like create a virtual machine, start it, stop it, delete it.

## Installation

run the following command

`
curl -s https://raw.githubusercontent.com/Fred78290/vmware-desktop-autoscaler-utility/main/install.sh | sh -`
`

The script will install vmware-desktop-autoscaler-utility into /usr/local/bin, generate certificates and install the service to be launched on login. It also install vmrest service.
