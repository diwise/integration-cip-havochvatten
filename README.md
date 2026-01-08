# integration-cip-havochvatten

An integration service that fetches temperature data from the Swedish Agency for Marine and Water Management (HaV) Badplatsen API and forwards it to either an LwM2M endpoint or an NGSI-LD Context Broker.

## Overview

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    integration-cip-havochvatten                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────┐    ┌──────────────┐    ┌─────────────────────────────┐ │
│  │   main.go   │───▶│ havochvatten │───▶│  Output (lwm2m or cip)      │ │
│  │  (CLI/Env)  │    │   (client)   │    │                             │ │
│  └─────────────┘    └──────────────┘    └─────────────────────────────┘ │
│        │                   │                         │                  │
└────────┼───────────────────┼─────────────────────────┼──────────────────┘
         │                   │                         │
         ▼                   ▼                         ▼
  ┌─────────────┐   ┌────────────────┐   ┌─────────────────────────────┐
  │  Nutscodes  │   │  Badplatsen    │   │  LwM2M Endpoint (primary)   │
  │  (input)    │   │  API (HaV)     │   │  or Context Broker          │
  └─────────────┘   └────────────────┘   └─────────────────────────────┘
```

### Data Flow

1. **Input**: Nutscodes are read from command line arguments, environment variable, or file
2. **Fetching**: For each nutscode, the following is retrieved:
   - `BathWaterProfile` - bathing site profile with Copernicus/SMHI forecasts
   - `Detail` - current sampling data with measured temperature
3. **Transformation**: Data is converted to `Temperature` objects
4. **Output**: Data is sent to the selected endpoint

### External API Calls

#### Badplatsen API (Swedish Agency for Marine and Water Management)

| Endpoint | Description |
|----------|-------------|
| `GET /detail` | Retrieves list of all bathing sites |
| `GET /detail/{nutsCode}` | Retrieves details for a specific bathing site |
| `GET /testlocationprofile/{nutsCode}` | Retrieves bathing water profile with forecast data |

**Data sources in response:**
- **Sample temperature**: Measured water temperature from the latest sampling
- **Copernicus/SMHI**: Modeled water temperature and weather data

### Output Formats

#### LwM2M (Primary Output)

Temperature data is serialized as SenML and sent to an LwM2M endpoint.

**LwM2M Object**: `3303` (Temperature Sensor)
**URN**: `urn:oma:lwm2m:ext:3303`

**SenML package example:**
```json
[
  {
    "bn": "{device_id}/3303/",
    "bt": 1704700800,
    "n": "0",
    "vs": "urn:oma:lwm2m:ext:3303"
  },
  {
    "n": "5700",
    "v": 18.5,
    "u": "Cel"
  }
]
```

| Resource | ID | Description |
|----------|-----|-------------|
| Object URN | 0 | Identifies the object type |
| Sensor Value | 5700 | Temperature value in Celsius |

**HTTP request:**
- **Method**: `POST`
- **Content-Type**: `application/vnd.oma.lwm2m.3303+json`
- **Expected response**: `201 Created`

#### NGSI-LD / FIWARE (Alternative Output)

Data is sent as `WaterQualityObserved` entities to an NGSI-LD Context Broker.

**Entity type**: `WaterQualityObserved`

**Attributes:**
| Attribute | Type | Description |
|----------|-----|-------------|
| `location` | GeoProperty | Coordinates (lat/lon) |
| `dateObserved` | Property | Time of observation |
| `temperature` | Property | Water temperature in Celsius |
| `source` | Property | Data source (API URL) |

## Configuration

### Environment Variables

#### CONTEXT_BROKER_URL

URL to NGSI-LD Context Broker.

#### LWM2M_ENDPOINT_URL

URL to endpoint capable of receiving LwM2M objects serialized as SenML (application/senml+json).

#### TLS_SKIP_VERIFY

Configure whether TLS certificates should be verified (default) or not. Valid values are "0" or "1".

#### HOV_BADPLATSEN_URL

URL to the Swedish Agency for Marine and Water Management API. Default: https://badplatsen.havochvatten.se/badplatsen/api

### Command Line Arguments

#### nutscodes

Specify which nutscodes to fetch from Hav och Vatten.

    -nutscodes=SE0A21480000004452,SE0A21480000000519,...

Optionally, you can specify which internal ID to use:

    -nutscodes=SE0A21480000004452=internal1,SE0A21480000000519=internal2,...

#### input

Specify a file containing nutscodes (one per line).

    -input=nutscodes.txt

#### output

Specify which output endpoint to use, `lwm2m` or `fiware`.

    -output=lwm2m

Sends LwM2M data to `LWM2M_ENDPOINT_URL`.

    -output=fiware

Sends FIWARE data to `CONTEXT_BROKER_URL`.

## Example

```bash
./integration-cip-havochvatten -nutscodes=SE0A21480000004452 -output=lwm2m
```
