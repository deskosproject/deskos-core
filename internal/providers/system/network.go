package system

import (
	"crypto/sha1"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
	"github.com/deskosproject/deskos-core/internal/providers/assets"
)

var NetworkProfileGVK = model.GVK{Group: "system.deskos.org", Version: model.V1Alpha1, Kind: "NetworkProfile"}

// DomainNetworkProfiles holds the 802.1X profiles, one per connection name.
var DomainNetworkProfiles = compose.Domain{ID: "network-profile", Label: "network profile"}

// NetworkProfilesDir is where NetworkManager reads connection keyfiles. It is a
// file the compiler generates, owned by root and mode 0600, with no secret in it
// (docs/adr/0027).
const NetworkProfilesDir = "/etc/NetworkManager/system-connections"

// NetworkCertsDir is where a profile's CA certificate is placed. It is the
// platform's PKI tree, whose files carry cert_t: NetworkManager may read them and
// a private key placed in the same tree is not left with the generic etc_t a new
// directory would get.
const NetworkCertsDir = "/etc/pki/tls/certs"

// NetworkProfileSpec is the public spec: a typed subset of an 802.1X connection.
type NetworkProfileSpec struct {
	Type        string `json:"type"`
	SSID        string `json:"ssid,omitempty"`
	EAP         string `json:"eap"`
	CACert      string `json:"caCert,omitempty"`
	ClientCert  string `json:"clientCert,omitempty"`
	PrivateKey  string `json:"privateKey,omitempty"`
	Identity    string `json:"identity,omitempty"`
	Secret      string `json:"secret,omitempty"`
	ServerName  string `json:"serverName,omitempty"`
	Autoconnect *bool  `json:"autoconnect,omitempty"`
	Priority    *int   `json:"priority,omitempty"`
}

// NetworkProfileObject is the decoded profile with its certificate read.
type NetworkProfileObject struct {
	Spec   NetworkProfileSpec
	CACert *assets.Asset
}

// NetworkProfile is one composed profile, keyed by connection name.
type NetworkProfile struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	SSID        string `json:"ssid,omitempty"`
	EAP         string `json:"eap"`
	CACertPath  string `json:"caCertPath,omitempty"`
	CACertSHA   string `json:"caCertSha256,omitempty"`
	ClientCert  string `json:"clientCert,omitempty"`
	PrivateKey  string `json:"privateKey,omitempty"`
	Identity    string `json:"identity,omitempty"`
	Secret      string `json:"secret"`
	ServerName  string `json:"serverName,omitempty"`
	Autoconnect bool   `json:"autoconnect"`
	Priority    int    `json:"priority"`
	UUID        string `json:"uuid"`
	CACertFile  string `json:"-"`
}

var (
	networkNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	networkHostRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`)
	// A client certificate names the machine, so it is a path the fleet fills and
	// never an asset of the repository. The standard PKI tree is used because its
	// SELinux type is cert_t and NetworkManager is allowed to read it; a directory
	// of our own would be etc_t and would need a policy we ship (docs/adr/0027).
	networkCertRE = regexp.MustCompile(`^/etc/pki/tls/certs/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	networkKeyRE  = regexp.MustCompile(`^/etc/pki/tls/private/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	// The methods NetworkManager documents for 802-1x.eap.
	networkEAP = []string{"leap", "md5", "tls", "peap", "ttls", "sim", "fast", "pwd"}
)

type networkProfile struct{}

func (networkProfile) GVK() model.GVK { return NetworkProfileGVK }
func (networkProfile) Schema() string {
	return "system.deskos.org/v1alpha1/networkprofile.json"
}

func (networkProfile) Decode(res *model.Resource) error {
	var s NetworkProfileSpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	switch s.Type {
	case "ethernet", "wifi":
	default:
		errs.Add(model.Errorf(res, "type %q must be ethernet or wifi", s.Type))
	}
	switch {
	case s.Type == "wifi" && s.SSID == "":
		errs.Add(model.Errorf(res, "a wifi profile needs ssid"))
	case s.Type == "ethernet" && s.SSID != "":
		errs.Add(model.Errorf(res, "ssid is only read for a wifi profile"))
	}
	if !contains(networkEAP, s.EAP) {
		errs.Add(model.Errorf(res, "eap %q must be one of %s", s.EAP, strings.Join(networkEAP, ", ")))
	}
	switch s.EAP {
	case "tls":
		if !networkCertRE.MatchString(s.ClientCert) || !networkKeyRE.MatchString(s.PrivateKey) {
			errs.Add(model.Errorf(res, "eap tls needs clientCert under /etc/pki/tls/certs/ and privateKey under /etc/pki/tls/private/"))
		}
		if s.Secret != "" && s.Secret != "none" {
			errs.Add(model.Errorf(res, "eap tls takes secret none: a certificate needs no password"))
		}
	default:
		// Measured on NetworkManager 1.52 (EL10): a password profile without a
		// non-empty identity is refused at load time ("802-1x.identity: property
		// is missing"), so a keyfile without one would never appear in nmcli.
		if strings.TrimSpace(s.Identity) == "" {
			errs.Add(model.Errorf(res, "eap %s needs identity: NetworkManager refuses a password profile without one; EAP-TLS does not need it", s.EAP))
		}
		if s.ClientCert != "" || s.PrivateKey != "" {
			errs.Add(model.Errorf(res, "clientCert and privateKey are only read for eap tls"))
		}
		if s.Secret == "none" {
			errs.Add(model.Errorf(res, "secret none is only valid for eap tls"))
		}
		if s.Secret != "" && s.Secret != "agent" {
			errs.Add(model.Errorf(res, "secret %q must be agent or none", s.Secret))
		}
	}
	if s.ServerName != "" && !networkHostRE.MatchString(s.ServerName) {
		errs.Add(model.Errorf(res, "serverName %q must be a host name", s.ServerName))
	}
	if s.Priority != nil && (*s.Priority < -999 || *s.Priority > 999) {
		errs.Add(model.Errorf(res, "priority %d must be between -999 and 999", *s.Priority))
	}
	obj := &NetworkProfileObject{Spec: s}
	if s.CACert != "" {
		as, err := assets.Read(res, s.CACert)
		if err != nil {
			errs.Add(model.Errorf(res, "caCert: %v", err))
		} else if _, err := certificateSubject(as.Data); err != nil {
			errs.Add(model.Errorf(res, "caCert %s: %v", as.Path, err))
		} else {
			obj.CACert = as
		}
	}
	res.Object = obj
	return errs.Err()
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (networkProfile) Contribute(res *model.Resource, s *compose.Scope) error {
	obj := res.Object.(*NetworkProfileObject)
	spec := obj.Spec
	secret := spec.Secret
	if secret == "" {
		if spec.EAP == "tls" {
			secret = "none"
		} else {
			secret = "agent"
		}
	}
	p := NetworkProfile{
		Name: res.Metadata.Name, Type: spec.Type, SSID: spec.SSID, EAP: spec.EAP,
		ServerName: spec.ServerName, UUID: connectionUUID(res.Metadata.Name),
		Autoconnect: true, ClientCert: spec.ClientCert, PrivateKey: spec.PrivateKey,
		Identity: spec.Identity, Secret: secret,
	}
	if spec.Autoconnect != nil {
		p.Autoconnect = *spec.Autoconnect
	}
	if spec.Priority != nil {
		p.Priority = *spec.Priority
	}
	if obj.CACert != nil {
		p.CACertPath = path.Join(NetworkCertsDir, "deskos-"+p.Name+".crt")
		p.CACertSHA = obj.CACert.SHA256
		p.CACertFile = obj.CACert.File
	}
	s.AddKeyed(DomainNetworkProfiles, p.Name, p,
		fmt.Sprintf("%s on %s (%s, uuid %s)", p.Name, p.Type, p.EAP, p.UUID))
	return nil
}

// connectionUUID derives a stable UUID from the connection name, so two
// compositions of the same resource produce the same keyfile. A random UUID
// would make every build a change.
func connectionUUID(name string) string {
	// RFC 4122 version 5, namespace = the DeskOS DNS name.
	sum := sha1.Sum([]byte("deskos.org/network-profile\x00" + name))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// lowerNetwork writes each profile as a keyfile NetworkManager reads, and places
// the CA certificate beside it when the profile names one. It writes no secret:
// the password is marked agent-owned and the identity is left to the fleet
// (docs/adr/0027).
func lowerNetwork(c *compose.Composition, p *plan.Plan) error {
	profiles := c.Result.Keyed(DomainNetworkProfiles)
	if len(profiles) == 0 {
		return nil
	}
	for _, k := range profiles {
		np := k.Value.(NetworkProfile)
		if np.CACertPath != "" {
			p.Artifact.Files = append(p.Artifact.Files, plan.FileInstall{
				Path: np.CACertPath, Mode: "0644", SHA256: np.CACertSHA,
				Asset: np.Name + ".crt", AssetFile: np.CACertFile, Provenance: k.Provenance,
			})
		}
		p.Artifact.GeneratedFiles = append(p.Artifact.GeneratedFiles, plan.GeneratedFile{
			Path: path.Join(NetworkProfilesDir, np.Name+".nmconnection"), Mode: "0600",
			Content: networkKeyfile(np), Provenance: k.Provenance,
		})
	}
	return nil
}

// networkKeyfile is the keyfile NetworkManager's plugin reads. Only what the
// profile decides is written; the password flag makes NetworkManager ask an agent
// for the secret instead of storing one.
func networkKeyfile(np NetworkProfile) string {
	var b strings.Builder
	b.WriteString("# Generated by deskosctl. Do not edit; change the DeskOS resources instead.\n")
	b.WriteString("[connection]\n")
	fmt.Fprintf(&b, "id=%s\n", np.Name)
	fmt.Fprintf(&b, "uuid=%s\n", np.UUID)
	fmt.Fprintf(&b, "type=%s\n", np.Type)
	fmt.Fprintf(&b, "autoconnect=%t\n", np.Autoconnect)
	fmt.Fprintf(&b, "autoconnect-priority=%d\n", np.Priority)
	if np.Type == "wifi" {
		b.WriteString("\n[wifi]\n")
		fmt.Fprintf(&b, "ssid=%s\n", np.SSID)
	}
	b.WriteString("\n[802-1x]\n")
	fmt.Fprintf(&b, "eap=%s;\n", np.EAP)
	if np.Identity != "" {
		fmt.Fprintf(&b, "identity=%s\n", np.Identity)
	}
	switch np.Secret {
	case "none":
		// EAP-TLS: the certificate and key the fleet placed, and no secret.
		fmt.Fprintf(&b, "client-cert=%s\n", np.ClientCert)
		fmt.Fprintf(&b, "private-key=%s\n", np.PrivateKey)
	default:
		// A session agent supplies the password; the file carries none.
		b.WriteString("password-flags=1\n")
	}
	if np.CACertPath != "" {
		fmt.Fprintf(&b, "ca-cert=%s\n", np.CACertPath)
	} else {
		b.WriteString("system-ca-certs=true\n")
	}
	if np.ServerName != "" {
		fmt.Fprintf(&b, "domain-suffix-match=%s\n", np.ServerName)
	}
	b.WriteString("\n[ipv4]\nmethod=auto\n\n[ipv6]\nmethod=auto\n")
	return b.String()
}
