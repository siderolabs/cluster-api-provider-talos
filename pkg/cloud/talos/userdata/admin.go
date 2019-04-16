package userdata

//GenAdmin returns the talos admin conf
func GenAdmin(in *Input) (string, error) {
	return renderTemplate(in, adminTempl)
}

const adminTempl = `---
context: {{ .ClusterName }}
contexts:
  {{ .ClusterName }}:
    target: {{ index .MasterIPs 0 }}
    ca: {{ .Certs.OsCert }}
    crt: {{ .Certs.AdminCert }}
    key: {{ .Certs.AdminKey }}
`
