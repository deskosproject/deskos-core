package compiler_test

import (
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/plan"
)

func networkProfileYAML(name, body string) string {
	return "apiVersion: system.deskos.org/v1alpha1\nkind: NetworkProfile\nmetadata:\n  name: " + name + "\nspec:\n" + body
}

func TestNetworkProfileWritesAKeyfileWithoutSecrets(t *testing.T) {
	// A corporate 802.1X profile: the image carries the trust decision and the
	// policy, and no secret at all (docs/adr/0027).
	cert := testCertPEM(t, "Example Corp Root CA")
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "NetworkProfile/corp"),
		"corp.yaml": networkProfileYAML("corp", "  type: wifi\n  ssid: CorpWLAN\n  eap: peap\n  identity: deskos@example.com\n"+
			"  caCert: assets/corp-ca.pem\n  serverName: radius.example.com\n  priority: 10\n"),
		"assets/corp-ca.pem": cert,
	}).dir(t)
	p := mustPlan(t, "ws", dir)

	var keyfile *plan.GeneratedFile
	for i := range p.Artifact.GeneratedFiles {
		if strings.HasSuffix(p.Artifact.GeneratedFiles[i].Path, "/corp.nmconnection") {
			keyfile = &p.Artifact.GeneratedFiles[i]
		}
	}
	if keyfile == nil {
		t.Fatalf("no keyfile in the plan: %+v", p.Artifact.GeneratedFiles)
	}
	if keyfile.Mode != "0600" {
		t.Errorf("keyfile mode %s, want 0600", keyfile.Mode)
	}
	if keyfile.Path != "/etc/NetworkManager/system-connections/corp.nmconnection" {
		t.Errorf("keyfile path %s", keyfile.Path)
	}
	for _, want := range []string{
		"id=corp\n", "type=wifi\n", "ssid=CorpWLAN\n", "eap=peap;\n",
		"password-flags=1\n", "identity=deskos@example.com\n", "ca-cert=/etc/pki/tls/certs/deskos-corp.crt\n",
		"domain-suffix-match=radius.example.com\n", "autoconnect-priority=10\n",
	} {
		if !strings.Contains(keyfile.Content, want) {
			t.Errorf("keyfile lacks %q:\n%s", want, keyfile.Content)
		}
	}
	// No secret is written, and no identity: it belongs to the fleet.
	for _, forbidden := range []string{"password=", "private-key="} {
		if strings.Contains(keyfile.Content, forbidden) {
			t.Errorf("keyfile carries %q:\n%s", forbidden, keyfile.Content)
		}
	}
	// The CA certificate is placed as public material.
	var placed bool
	for _, f := range p.Artifact.Files {
		if f.Path == "/etc/pki/tls/certs/deskos-corp.crt" {
			placed = true
			if f.Mode != "0644" {
				t.Errorf("certificate mode %s, want 0644", f.Mode)
			}
		}
	}
	if !placed {
		t.Error("the CA certificate is not in the plan")
	}
}

func TestNetworkProfileTLSCarriesNoSecret(t *testing.T) {
	// EAP-TLS is the path that works before a login: a machine certificate the
	// fleet places, and no secret to ask anyone for (docs/adr/0027).
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "NetworkProfile/wired"),
		"wired.yaml": networkProfileYAML("wired", "  type: ethernet\n  eap: tls\n"+
			"  clientCert: /etc/pki/tls/certs/wired.crt\n  privateKey: /etc/pki/tls/private/wired.key\n"+
			"  serverName: radius.example.com\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	var content string
	for _, f := range p.Artifact.GeneratedFiles {
		if strings.HasSuffix(f.Path, "/wired.nmconnection") {
			content = f.Content
		}
	}
	for _, want := range []string{
		"eap=tls;\n", "client-cert=/etc/pki/tls/certs/wired.crt\n",
		"private-key=/etc/pki/tls/private/wired.key\n", "system-ca-certs=true\n",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("keyfile lacks %q:\n%s", want, content)
		}
	}
	// Nothing is asked for, because nothing is needed.
	if strings.Contains(content, "password-flags") {
		t.Errorf("a TLS profile asks for a password:\n%s", content)
	}
	// The certificate is not in the image: it names the machine.
	for _, f := range p.Artifact.Files {
		if strings.Contains(f.Path, "/etc/deskos/network/") {
			t.Errorf("the machine certificate was placed by the build: %s", f.Path)
		}
	}
}

func TestNetworkProfileSecretRules(t *testing.T) {
	load := func(body string) error {
		dir := base.with(fixture{
			"ws.yaml":   workstation("ws", "core"),
			"core.yaml": profile("core", "foundation", "NetworkProfile/p"),
			"p.yaml":    networkProfileYAML("p", body),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		return err
	}
	// A password method defaults to the session agent, which works after a login.
	if err := load("  type: wifi\n  ssid: CorpWLAN\n  eap: peap\n  identity: deskos@example.com\n"); err != nil {
		t.Fatalf("a PEAP profile is valid: %v", err)
	}
	mustFail(t, load("  type: wifi\n  ssid: CorpWLAN\n  eap: peap\n  identity: deskos@example.com\n  secret: none\n"), "only valid for eap tls")
	mustFail(t, load("  type: ethernet\n  eap: peap\n  identity: a\n  clientCert: /etc/pki/tls/certs/a.crt\n"), "'not' failed")
	mustFail(t, load("  type: ethernet\n  eap: tls\n"), "missing properties")
	mustFail(t, load("  type: ethernet\n  eap: tls\n  clientCert: /tmp/a.crt\n  privateKey: /tmp/a.key\n"), "does not match pattern")
	mustFail(t, load("  type: ethernet\n  eap: tls\n  clientCert: /etc/pki/tls/certs/a.crt\n  privateKey: /etc/pki/tls/private/a.key\n  secret: agent\n"), "secret none")
	mustFail(t, load("  type: wifi\n  ssid: CorpWLAN\n  eap: peap\n"), "missing property 'identity'")
}

func TestNetworkProfileWithoutACertUsesTheSystemStore(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml":    workstation("ws", "core"),
		"core.yaml":  profile("core", "foundation", "NetworkProfile/wired"),
		"wired.yaml": networkProfileYAML("wired", "  type: ethernet\n  eap: peap\n  identity: deskos@example.com\n  autoconnect: false\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	var content string
	for _, f := range p.Artifact.GeneratedFiles {
		if strings.HasSuffix(f.Path, "/wired.nmconnection") {
			content = f.Content
		}
	}
	if !strings.Contains(content, "system-ca-certs=true\n") {
		t.Errorf("no system trust store in the keyfile:\n%s", content)
	}
	if !strings.Contains(content, "autoconnect=false\n") {
		t.Errorf("autoconnect is not what the profile declared:\n%s", content)
	}
	if strings.Contains(content, "[wifi]") {
		t.Errorf("an ethernet profile has a wifi section:\n%s", content)
	}
}

func TestNetworkProfileUUIDIsStable(t *testing.T) {
	// Two compositions of the same resource produce the same keyfile: a random
	// UUID would make every build a change.
	build := func() string {
		dir := base.with(fixture{
			"ws.yaml":   workstation("ws", "core"),
			"core.yaml": profile("core", "foundation", "NetworkProfile/corp"),
			"corp.yaml": networkProfileYAML("corp", "  type: wifi\n  ssid: CorpWLAN\n  eap: peap\n  identity: deskos@example.com\n"),
		}).dir(t)
		p := mustPlan(t, "ws", dir)
		for _, f := range p.Artifact.GeneratedFiles {
			if strings.HasSuffix(f.Path, "/corp.nmconnection") {
				return f.Content
			}
		}
		return ""
	}
	first, second := build(), build()
	if first == "" || first != second {
		t.Fatalf("the keyfile is not deterministic:\n%s\n---\n%s", first, second)
	}
	if !strings.Contains(first, "uuid=") {
		t.Error("the keyfile has no uuid")
	}
}

func TestNetworkProfileIsChecked(t *testing.T) {
	load := func(body string) error {
		dir := base.with(fixture{
			"ws.yaml":   workstation("ws", "core"),
			"core.yaml": profile("core", "foundation", "NetworkProfile/p"),
			"p.yaml":    networkProfileYAML("p", body),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		return err
	}
	if err := load("  type: wifi\n  ssid: CorpWLAN\n  eap: peap\n  identity: deskos@example.com\n"); err != nil {
		t.Fatalf("a wifi profile is valid: %v", err)
	}
	// The schema rejects the shape first; the typed decoder repeats the rules so a
	// schema regression cannot weaken them.
	mustFail(t, load("  type: wifi\n  eap: peap\n"), "missing property 'ssid'")
	mustFail(t, load("  type: ethernet\n  ssid: CorpWLAN\n  eap: peap\n"), "'not' failed")
	mustFail(t, load("  type: wifi\n  ssid: CorpWLAN\n  eap: kerberos\n"), "must be one of")
	mustFail(t, load("  type: wifi\n  ssid: CorpWLAN\n  eap: peap\n  serverName: 'radius:1812'\n"), "does not match pattern")
	mustFail(t, load("  type: wifi\n  ssid: CorpWLAN\n  eap: peap\n  priority: 1000\n"), "maximum")
}

func TestNetworkProfileRendersInTheContainerfile(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "NetworkProfile/corp"),
		"corp.yaml": networkProfileYAML("corp", "  type: wifi\n  ssid: CorpWLAN\n  eap: peap\n  identity: deskos@example.com\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	files, err := containerfile.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	// Generated files reach the image through the rootfs the build copies, not as
	// a line in the Containerfile.
	var installed bool
	for _, f := range files {
		if strings.HasSuffix(f.Path, "/etc/NetworkManager/system-connections/corp.nmconnection") {
			installed = true
			if !strings.Contains(string(f.Data), "password-flags=1") {
				t.Errorf("the keyfile in the image is not the one the plan declares:\n%s", f.Data)
			}
		}
	}
	if !installed {
		t.Errorf("the keyfile is not in the build's rootfs: %+v", files)
	}
}
