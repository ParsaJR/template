package main

import (
	"os"
	"reflect"
	"testing"

	"github.com/libdns/liara"
	"github.com/libdns/libdns/libdnstest"
)

// Warning: This test will create/delete real DNS records prefixed with "test-".
// Use a dedicated test zone or ensure you have backups.
func TestLiaraDNSProvider(t *testing.T) {
	liaraAPIToken := os.Getenv("LIARA_API_TOKEN")

	testZone := os.Getenv("LIARA_TEST_ZONE")

	if liaraAPIToken == "" || testZone == "" {
		t.Fatal("Liara provider tests failed: LIARA_API_TOKEN & LIARA_TEST_ZONE environment variables must be set. ")
	}

	provider := &liara.Provider{
		APIToken: liaraAPIToken,
	}

	wrappedProvider := libdnstest.WrapNoZoneLister(provider)

	suite := libdnstest.NewTestSuite(wrappedProvider, testZone)

	// Skipping some records from the test. Liara doesn't seem to support them.
	suite.SkipRRTypes = map[string]bool{
		"CAA":   true,
		"HTTPS": true,
		"NS":    true,
		"SVCB":  true,
	}
	suite.RunTests(t)
}

func TestRemainingContent(t *testing.T) {

	existing := []liara.APIRecordContent{
		{
			IP: "192.168.1.1",
		},
		{
			IP: "127.0.0.1",
		},
	}

	subtract := []liara.APIRecordContent{
		{
			IP: "127.0.0.1",
		},
	}

	expected := []liara.APIRecordContent{
		{
			IP: "192.168.1.1",
		},
	}

	got := liara.RemainingContent(existing, subtract)

	if len(got) != 1 {
		t.Errorf("Expected one item as a result, got %d", len(got))
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Unexpected result. Expected %v but got %v", expected, got)
	}

}

func TestRemainingContentRemoveAll(t *testing.T) {
	existing := []liara.APIRecordContent{
		{IP: "1.1.1.1"},
		{IP: "2.2.2.2"},
	}

	got := liara.RemainingContent(existing, existing)

	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}
