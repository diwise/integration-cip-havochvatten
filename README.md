# integration-cip-havochvatten

## Configuration

### Environment variables

#### CONTEXT_BROKER_URL

Url to NGSI-LD Context Broker

#### LWM2M_ENDPOINT_URL

Url to endpoint capable of receiving lwm2m object serialized as senML (application/senml+json) 

#### TLS_SKIP_VERIFY

Configure if TLS certificates should be verified (default) or not. Valid values are "0" or "1".

#### HOV_BADPLATSEN_URL

Url to Hav och Vatten API, default https://badplatsen.havochvatten.se/badplatsen/api

### Command line arguments

#### nutscodes

Specify which nutscodes that should be loaded from Hav och Vatten.

    -nutscodes=SE0A21480000004452,SE0A21480000000519,...

#### output

Specify which output endpoint that should be used, `lwm2m` or `fiware`.

    -output=lwm2m

send lwm2m data  to `LWM2M_ENDPOINT_URL`

    -output=fiware

send fiware data to `CONTEXT_BROKER_URL`

## Example

```bash
./integration-cip-havochvatten -nutscodes=SE0A21480000004452 -output=lwm2m
```
