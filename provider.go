// Package liara implements a DNS record management client compatible
// with the libdns interfaces for Liara.
package liara

import (
	"context"
	"fmt"

	"github.com/libdns/libdns"
)

// Provider facilitates DNS record manipulation with Liara
type Provider struct {
	APIToken string `json:"api_token,omitempty"`
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	client := newClient(p.APIToken)

	records, err := client.APIGetRecords(ctx, zone)
	if err != nil {
		return nil, err
	}

	libdns_records := make([]libdns.Record, 0)

	for _, record := range records {
		libdns_record, err := record.ToLibdnsRRType(zone)
		if err != nil {
			return nil, err
		}

		libdns_records = append(libdns_records, libdns_record...)
	}

	return libdns_records, nil
}

// AppendRecords adds records to the zone. It returns the records that were
// added in a form of libdns concrete type, and not a opaque RR type.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	client := newClient(p.APIToken)

	added_records := make([]libdns.Record, 0, len(records))

	liaraRecords, err := ToLiaraAPIRecords(records, zone)
	if err != nil {
		return nil, err
	}

	for _, record := range liaraRecords {
		added, err := client.APIPostRecord(ctx, zone, record)
		if err != nil {
			return nil, fmt.Errorf("Posting the record to Liara failed: %s", err)
		}

		addedAsLibdns, err := added.ToLibdnsRRType(zone)

		if err != nil {
			return nil, fmt.Errorf("Conversion to libdns type failed: %s", err)
		}

		added_records = append(added_records, addedAsLibdns...)
	}
	fmt.Println(len(added_records), " Has been appended: ", added_records)
	fmt.Println()

	return added_records, nil
}

// Here, the "SetRecords" method doesn't provide atomicity. Meaning a non nil value for the returned
// "error" can indicate that the zone is in a invalid state, and there is
// no rollback operation happened to rollback the previous changes.
//
// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records, And not the skipped ones.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	fmt.Println("Hello SetRecords with content ", records)
	existingRecords, err := p.GetRecords(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("Failure when fetching the record from Liara: %s", err)
	}

	toDelete := make([]libdns.Record, 0)

	for _, new := range records {
		exists := libDNSContains(existingRecords, new)
		if exists != nil {
			toDelete = append(toDelete, exists...)
		}
	}
	_, err = p.DeleteRecords(ctx, zone, toDelete)

	if err != nil {
		return nil, fmt.Errorf("Deleting records from Liara has failed: %s", err)
	}

	return p.AppendRecords(ctx, zone, records)

}

// DeleteRecords deletes the specified records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	fmt.Println("Hello DeleteRecords with content ", records)
	fmt.Println()
	client := newClient(p.APIToken)

	deleted := make([]libdns.Record, 0, len(records))

	existingRecords, err := client.APIGetRecords(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch the dns records form Liara: %s", err)
	}

	for _, record := range records {

		toRemoveLiara, err := ToLiaraAPIRecord(record, zone)
		if err != nil {
			return nil, fmt.Errorf("Failed to convert libdns type to APIRecord: %s", err)
		}

		existingLiara, yes := APIRecordContains(existingRecords, toRemoveLiara)

		if !yes {
			// Silently skipping the delete request, for a non-existent record.
			continue
		}

		// Check to see how much of the content slice remains after deletion.
		remaining := RemainingContent(
			existingLiara.Contents,
			toRemoveLiara.Contents,
		)

		fmt.Println("----Check for ", existingLiara.Contents, " AND ",
			toRemoveLiara.Contents, "Resulted INNN ", remaining)

		if len(remaining) == 0 {
			// remaning 0  means the entire record should be deleted.
			fmt.Println("Going to remove a record ", existingLiara)
			err = client.APIDeleteRecord(ctx, zone, existingLiara.ID)
			if err != nil {
				return nil, fmt.Errorf("Error occured when deleting a record from Liara: %s", err.Error())
			}

		} else {
			// remaning != 0 means the record has to be updated with new
			// remaining values.
			// E.x. example.com | 1.1.1.1, 1.0.0.1 => example.com | 1.1.1.1

			existingLiara.Contents = remaining

			updatedRecord := *existingLiara
			updatedRecord.ID = ""

			_, err = client.APIUpdateRecord(
				ctx,
				zone,
				existingLiara.ID,
				updatedRecord,
			)

			if err != nil {
				return nil, fmt.Errorf(
					"error occurred when updating a record through Liara: %w",
					err,
				)
			}

		}

		converted, err := toRemoveLiara.ToLibdnsRRType(zone)
		if err != nil {
			return nil, err
		}

		deleted = append(deleted, converted...)
	}

	return deleted, nil
}

// Differentiates Remaining content in "APIRecord.Content".
// Takes the existing APIRecord.Contents and removes the contents that the user asked to delete.
func RemainingContent(
	existing []APIRecordContent,
	recordsToRemove []APIRecordContent,
) []APIRecordContent {

	remaining := make([]APIRecordContent, 0, len(existing))

	for _, record := range existing {
		found := false

		for _, recordToRemove := range recordsToRemove {
			if record.IsEqual(recordToRemove) {
				found = true
				break
			}
		}

		if !found {
			remaining = append(remaining, record)
		}

	}

	return remaining
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
