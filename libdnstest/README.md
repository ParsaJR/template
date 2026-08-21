# Provider-Specific Tests for Liara

This directory contains provider-specific tests for the Liara libdns provider
using the official [libdnstest package](https://github.com/libdns/libdns/tree/master/libdnstest).
These tests verify the provider implementation against the real [Liara API](https://openapi.liara.ir/?urls.primaryName=DNS#/), ensuring all
libdns interface methods work correctly with actual DNS operations.

> [!WARNING]
> When testing **real DNS providers** run the tests on dedicated test zones. **Your DNS records may be deleted or overwritten.** Even though tests use "test-" prefixed record names, bugs in the provider or test framework could cause additional data loss.


## How To Run

1. **Get API Token and setup zone**: You can obtain the api-key and setup a test
   zone form the [Liara's console](https://console.liara.ir/).
   
   
2. **Set Environment Variables**:

```bash
export LIARA_API_TOKEN="your-token-here" 
export LIARA_TEST_ZONE="example.org."  # Include trailing dot
```

3. **Run Tests**

```bash
go test -v
```


## What Gets Tested

- GetRecords, AppendRecords, SetRecords, DeleteRecords
- Complete record lifecycle (create → update → delete)
- Various DNS record types
- Some internal utilities 
