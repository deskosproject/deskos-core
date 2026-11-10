package system

import (
	"strings"
	"testing"
)

func TestCertificateSubjectRejectsPrivateKeys(t *testing.T) {
	data := []byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n")
	_, err := certificateSubject(data)
	if err == nil || !strings.Contains(err.Error(), "only PEM CERTIFICATE") {
		t.Fatalf("error = %v", err)
	}
}

func TestCertificateSubjectRejectsTrailingData(t *testing.T) {
	_, err := certificateSubject([]byte("not a certificate\n"))
	if err == nil {
		t.Fatal("non-PEM data was accepted")
	}
}
