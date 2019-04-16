package userdata

import (
	"bytes"
	"errors"
	"text/template"
)

//CertStrings holds a
type CertStrings struct {
	Crt string
	Key string
}

//Input holds info about certs, ips, and node type
type Input struct {
	UserdataType  string //init,controlplane, or worker
	Certs         *Certs
	MasterIPs     []string
	Index         int
	ClusterName   string
	ServiceDomain string
	PodNet        []string
	ServiceNet    []string
	Endpoints     string
	KubeadmTokens *KubeadmTokens
	TrustdInfo    *TrustdInfo
}

//Certs holds a bunch of b64 encoded cert strings
type Certs struct {
	AdminCert string
	AdminKey  string
	OsCert    string
	OsKey     string
	K8sCert   string
	K8sKey    string
}

//KubeadmTokens holds token strings
type KubeadmTokens struct {
	BootstrapToken string
	CertKey        string
}

//TrustdInfo holds the trustd creds
type TrustdInfo struct {
	Username string
	Password string
}

//GenUdata will return the talos userdata for a given node type
func GenUdata(in *Input) (string, error) {
	err := errors.New("")
	templateData := ""
	switch udtype := in.UserdataType; udtype {
	case "init":
		templateData = initTempl
	case "controlplane":
		templateData = controlPlaneTempl
	case "worker":
		templateData = workerTempl
	default:
		return "", errors.New("Unable to determine userdata type to generate")
	}

	ud, err := renderTemplate(in, templateData)
	if err != nil {
		return "", err
	}
	return ud, nil
}

//renderTemplate will output a templated string
func renderTemplate(in *Input, udTemplate string) (string, error) {
	templ := template.Must(template.New("udTemplate").Parse(udTemplate))
	var buf bytes.Buffer
	if err := templ.Execute(&buf, in); err != nil {
		return "", err
	}
	return buf.String(), nil
}
