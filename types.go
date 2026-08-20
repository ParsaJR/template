package liara

import (
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/libdns/libdns"
)

// The generic api error schema of Liara's dns api.
// E.x.
// StatusCode: 400
// StatusMessage: "Bad Request"
// ErrorMessage: "invalid record type."
type APIError struct {
	StatusCode    int    `json:"statusCode"`
	StatusMessage string `json:"error"`
	ErrorMessage  string `json:"message"`
}

// Liara's response model. The same model is also used for its PUT operation.
type PostRecordResponse struct {
	Status string    `json:"status"`
	Data   APIRecord `json:"data"`
}

type GetRecordsResponse struct {
	Status string      `json:"status"`
	Data   []APIRecord `json:"data"`
}

type APIRecord struct {
	// omitting the "id" field is a requirement in PUT operations.
	ID       string             `json:"id,omitempty"`
	Name     string             `json:"name"`
	Type     string             `json:"type"` // A, AAAA, CNAME, MX, SRV, TXT
	TTL      int                `json:"ttl"`
	Contents []APIRecordContent `json:"contents"`
}

type APIRecordContent struct {
	IP string `json:"ip,omitempty"`

	// Relevant to the CNAME record type
	Host string `json:"host,omitempty"`

	// Relevant to the MX And Srv record types.
	Priority uint16 `json:"priority,omitempty"`
	Weight   uint16 `json:"weight,omitempty"`
	Port     uint16 `json:"port,omitempty"`

	// Relevant to the TXT record types.
	Text string `json:"text,omitempty"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("%d: %s", e.StatusCode, e.ErrorMessage)
}

// Checks to see if two APIRecordContent structs are identical.
func (current APIRecordContent) IsEqual(target APIRecordContent) bool {
	return current.IP == target.IP &&
		current.Host == target.Host &&
		current.Priority == target.Priority &&
		current.Weight == target.Weight &&
		current.Port == target.Port &&
		current.Text == target.Text
}

// Converts the Liara specific "APIRecord" type to an appropriate, concrete RR
// type struct in libdns(the types are in the file "rrtypes.go").
//
// It returns an slice(mostly a single item slice), because an APIRecord can
// consist of multiple "Contents" which doesn't fit into a single libdns type.
func (r APIRecord) ToLibdnsRRType(zone string) ([]libdns.Record, error) {

	// libdns representation of the "Name", is a relative name to the zone.
	//
	// E.g.
	// Liara representation: test-append-address.suckless.ir
	// Libdns representation: test-append-address
	// So:
	name := libdns.RelativeName(r.Name, zone)

	ttl := time.Duration(r.TTL) * time.Second

	switch strings.ToUpper(r.Type) {
	case "A", "AAAA":
		records := make([]libdns.Record, 0, len(r.Contents))

		for _, content := range r.Contents {
			ip, err := netip.ParseAddr(content.IP)
			if err != nil {
				return nil, fmt.Errorf("invalid IP address %q", content.IP)
			}

			records = append(records, libdns.Address{
				Name: name,
				TTL:  ttl,
				IP:   ip,
			})
		}

		return records, nil

	case "CNAME":
		if len(r.Contents) != 1 {
			return nil, fmt.Errorf(
				"CNAME record %q has %d contents, expected 1",
				r.Name,
				len(r.Contents),
			)
		}

		return []libdns.Record{
			libdns.CNAME{
				Name:   name,
				TTL:    ttl,
				Target: r.Contents[0].Host + ".",
			},
		}, nil

	case "MX":
		records := make([]libdns.Record, 0, len(r.Contents))

		for _, content := range r.Contents {
			records = append(records, libdns.MX{
				Name:       name,
				TTL:        ttl,
				Preference: content.Priority,
				Target:     content.Host + ".",
			})
		}

		return records, nil

	case "SRV":
		records := make([]libdns.Record, 0, len(r.Contents))

		for _, content := range r.Contents {
			// Going to parse the SRV record according to
			// https://en.wikipedia.org/wiki/SRV_record
			// r.Name is from the expected Liara's format:
			// _newservice._tcp.test-set-srv.example.ir
			parts := strings.Split(r.Name, ".")

			if len(parts) < 3 ||
				!strings.HasPrefix(parts[0], "_") ||
				!strings.HasPrefix(parts[1], "_") {
				return nil, fmt.Errorf("invalid SRV record name %q", r.Name)
			}

			service := strings.TrimPrefix(parts[0], "_")   // The service name, like "_sip"
			transport := strings.TrimPrefix(parts[1], "_") // The protocol of service, like "_udp"

			name := strings.Join(parts[2:], ".")
			name = libdns.RelativeName(name, zone)

			records = append(records, libdns.SRV{
				Name:      name,
				Service:   service,
				Transport: transport,
				TTL:       ttl,
				Priority:  content.Priority,
				Target:    content.Host + ".",
				Weight:    content.Weight,
				Port:      content.Port,
			})
		}

		return records, nil
	case "TXT":
		records := make([]libdns.Record, 0, len(r.Contents))

		for _, content := range r.Contents {
			record := libdns.TXT{
				Name: name,
				TTL:  ttl,
				Text: content.Text,
			}
			records = append(records, record)
		}

		return records, nil

	default:
		return nil, fmt.Errorf("Got an unsupported record type %v", r)
	}
}

// turns a flat []libdns.Record into Liara's []APIRecord, where records with the
// same name + type can possibly share one APIRecord with multiple contents.
func ToLiaraAPIRecords(records []libdns.Record, zone string) ([]APIRecord, error) {
	groups := make(map[string]*APIRecord)

	for _, r := range records {
		var (
			name       string
			recordType string
			ttl        int
			content    APIRecordContent
		)

		name = libdns.AbsoluteName(r.RR().Name, zone)

		switch record := r.(type) {
		case libdns.Address:
			recordType = "A"
			if record.IP.Is6() {
				recordType = "AAAA"
			}
			ttl = int(record.TTL.Seconds())

			content.IP = record.IP.String()

		case libdns.CNAME:
			recordType = "CNAME"
			ttl = int(record.TTL.Seconds())

			content.Host = strings.TrimSuffix(record.Target, ".")

		case libdns.MX:
			recordType = "MX"
			ttl = int(record.TTL.Seconds())

			content.Host = strings.TrimSuffix(record.Target, ".")
			content.Priority = record.Preference

		case libdns.SRV:
			recordType = "SRV"
			ttl = int(record.TTL.Seconds())

			content.Host = strings.TrimSuffix(record.Target, ".")
			content.Priority = record.Priority
			content.Weight = record.Weight
			content.Port = record.Port

		case libdns.TXT:
			recordType = "TXT"
			ttl = int(record.TTL.Seconds())

			content.Text = record.Text

		case libdns.RR:
			recordType = strings.ToUpper(record.Type)
			ttl = int(record.TTL.Seconds())

			if recordType != "TXT" {
				return nil, fmt.Errorf(
					"unsupported generic record type %q",
					record.Type,
				)
			}

			content.Text = record.Data

		default:
			return nil, fmt.Errorf("unsupported record type %T", r)
		}

		key := name + ":" + recordType

		// Check if the same type of record exists in the previous groups, and
		// if there is, append the content to its Contents slice.
		if existing, ok := groups[key]; ok {
			existing.Contents = append(existing.Contents, content)
			continue
		}

		groups[key] = &APIRecord{
			Name: name,
			Type: recordType,
			TTL:  ttl,
			Contents: []APIRecordContent{
				content,
			},
		}
	}

	result := make([]APIRecord, 0, len(groups))
	for _, record := range groups {
		result = append(result, *record)
	}

	return result, nil
}

func ToLiaraAPIRecord(r libdns.Record, zone string) (APIRecord, error) {

	name := libdns.AbsoluteName(r.RR().Name, zone)

	switch record := r.(type) {
	case libdns.Address:
		recordType := "A"
		if record.IP.Is6() {
			recordType = "AAAA"
		}

		return APIRecord{
			Name: name,
			Type: recordType,
			TTL:  int(record.TTL.Seconds()),
			Contents: []APIRecordContent{
				{
					IP: record.IP.String(),
				},
			},
		}, nil

	case libdns.CNAME:
		return APIRecord{
			Name: name,
			Type: "CNAME",
			TTL:  int(record.TTL.Seconds()),
			Contents: []APIRecordContent{
				{
					Host: strings.TrimSuffix(record.Target, "."),
				},
			},
		}, nil

	case libdns.MX:
		return APIRecord{
			Name: name,
			Type: "MX",
			TTL:  int(record.TTL.Seconds()),
			Contents: []APIRecordContent{
				{
					Host:     strings.TrimSuffix(record.Target, "."),
					Priority: record.Preference,
				},
			},
		}, nil

	case libdns.SRV:
		return APIRecord{
			Name: name,
			Type: "SRV",
			TTL:  int(record.TTL.Seconds()),
			Contents: []APIRecordContent{
				{
					Host:     strings.TrimSuffix(record.Target, "."),
					Priority: record.Priority,
					Weight:   record.Weight,
					Port:     record.Port,
				},
			},
		}, nil

	case libdns.TXT:
		return APIRecord{
			Name: name,
			Type: "TXT",
			TTL:  int(record.TTL.Seconds()),
			Contents: []APIRecordContent{
				{
					Text: record.Text,
				},
			},
		}, nil

	// This is a special case. libdnstest inserts a case by using RR opaque type
	case libdns.RR:
		recordType := strings.ToUpper(record.Type)
		ttl := int(record.TTL.Seconds())

		if recordType != "TXT" {
			return APIRecord{}, fmt.Errorf(
				"unsupported opaque record type %q",
				record.Type,
			)
		}
		return APIRecord{
			Name: name,
			Type: recordType,
			TTL:  ttl,
			Contents: []APIRecordContent{
				{
					Text: record.Data,
				},
			},
		}, nil

	default:
		return APIRecord{}, fmt.Errorf("unsupported record type %T", r)
	}
}

func APIRecordContains(records []APIRecord, target APIRecord) (*APIRecord, bool) {
	for i := range records {
		if records[i].Name == target.Name &&
			records[i].Type == target.Type {
			return &records[i], true
		}
	}

	return nil, false
}

func libDNSContains(records []libdns.Record, target libdns.Record) []libdns.Record {
	recordsFound := make([]libdns.Record, 0)
	for i := range records {
		if records[i].RR().Name == target.RR().Name &&
			records[i].RR().Type == target.RR().Type {
			recordsFound = append(recordsFound, records[i])
		}
	}

	if len(recordsFound) != 0 {
		return recordsFound
	}

	return nil
}
