package userdata

const controlPlaneTempl = `---
version: ""
security: null
services:
  init:
    cni: flannel
  kubeadm:
    certificateKey: '{{ .KubeadmTokens.CertKey }}'
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: JoinConfiguration
      controlPlane:
        apiEndpoint:
          advertiseAddress: {{ index .MasterIPs .Index }}
          bindPort: 6443
      discovery:
        bootstrapToken:
          token: '{{ .KubeadmTokens.BootstrapToken }}'
          unsafeSkipCAVerification: true
          apiServerEndpoint: {{ index .MasterIPs 0 }}:443
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
          feature-gates: ExperimentalCriticalPodAnnotation=true
  trustd:
    username: '{{ .TrustdInfo.Username }}'
    password: '{{ .TrustdInfo.Password }}'
    endpoints: [ "{{ index .MasterIPs 0 }}" ]
    bootstrapNode: "{{ index .MasterIPs 0 }}"
    certSANs: [ "{{ index .MasterIPs .Index }}" ]
`
