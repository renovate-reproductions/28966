Backend API Documentation for Distributors
==========================================

Distributors are standalone executables that communicate with the rdsys backend to receive updated information on resources. This documentation specifies the form that this IPC takes.

The recommended way to receive timely updates from the backend is to open and maintain a presistent HTTP connection to the backend api `resource-stream` endpoint. The backend will periodically issue "resouce diffs" with relevant information on new, changed, or removed resources. Alternatively, distributors can use the `resources` endpoint to either `GET` a full list of resources for a given distributor or `POST` to add new resources to the backend.

### Initiating a resource stream
The resource stream is initiated by the distributor by making a `GET` request to the `resource-stream` endpoint with data:

`GET /resource-stream HTTP/1.1`

##### Headers
- `Host:` must be set
- `Authorization: Bearer [token]` must be set to the API bearer token
- `Content-Length:` must be set to the length of the supplied data for GET requests

##### Data

Distributors must send a JSON object with the following data:
```
{
  "request_origin": string,
  "resouce_types": [string]
}
```
where:
- `request_origin` is a string with the name of the distributor. This must correspond to a known distributor, specified in the config file for the rdsys backend.
- `resource_types` is a list of strings of requested resource types (e.g., "vanilla", "obfs4", "snowflake", etc.). Unknown resource types will be ignored.

<details>
<summary>Example:</summary>

```
GET /resource-stream HTTP/1.1
Host: localhost:7100
Authorization: Bearer HttpsApiTokenPlaceholder
Content-Type: application/json
Content-Length: 68

{"request_origin":"https","resource_types":["obfs2","scramblesuit"]}
```

</details>

### Response 
The HTTP response to the resource-stream API call is a chunked transfer encoding of JSON objects that represent a resouce diff. One is sent immediately and subsequent chunks are sent periodically when new information is available from the backend. Each diff is delimited with a carriage return character `\r`.

The first resource diff in every new stream connection will always contain a full update of all available resources for that distributor in the `new` field of the diff. Subsequent diffs *in the same connection* are updates on top of the first one. That is, there is no state stored between connections and if the HTTP connection ends, a new connection to the `resource-stream` endpoint will again begin with a full update of all available resources.

##### Bridge/Transport Resouce Diff JSON Object

```
{
  "new": {
    ResourceType: [
      Resource,
      Resource,
      ...,
      Resource
    ],
  },
  "changed": {
    ResourceType: [
      Resource,
      Resource,
      ...,
      Resource
    ],
  },
  "gone": {
    ResourceType: [
      Resource,
      Resource,
      ...,
      Resource
    ],
  }
  "full_update": bool
}
```

where:
- `new` are resources that have not yet been distributed. The first resource diff received when initiating the resource-stream API call will always only contain `new` resources.
- `changed` are resources that have previously been sent during this stream but have updated information.
- `gone` are resources that are no longer available and should not be given out by the distributor.
- `ResourceType` is a string with the name of the resouce type (e.g., "vanilla", "obfs4", "snowflake", etc.) the supplied resources should only include types that were incuded in the resource request.
- `Resouce` is a JSON object that represents the requested resouce (in this case a bridge):
   ```
   Resource = {
     "type" : string,
     "blocked_in" : {
        string : bool,
        string : bool
     },
     "location": {
       "countrycode" : string,
       "asn" : uint32
     },
     "protocol" : string,
     "address" : string,
     "port" : uint16,
     "fingerprint" : string,
     "or-addresses" : [string],
     "distribution" : string,
     "flags" : {
        "fast" : bool,
        "stable" : bool,
        "running" : bool,
        "valid" : bool,
     },
     "params" : {
       string : string,
       string : string
     },
   }
   ```
   where:
   - `type` is the name of the resouce type repeated here.
   - `blocked_in` is a map of string representations of locations to bools that indicate whether the resource is blocked in the indicated area.
   - `location` represents the physcal and topological location of the resource. This is usually null, but if not, contains the ISO 3166-1 alpha-2 country code, e.g. "AR" and/or the autonomous system number.
   - `protocol` is the transport layer protocol that the bridge supports (e.g., tcp or udp)
   - `address` is the main IP address associated with the bridge that users should connect to.
   - `port` is the port this bridge listens on
   - `fingerprint`
   - `or-addresses` are additional addresses that can be used to connect to this bridge. These are commonly IPv6 addresses if the bridge supports both IPv4 and IPv6.
   - `distribution` is the distribution method preference set by the bridge operator in their torrc configuration. This can be ignored by the distributor itself as it's mostly used for filtering at the backend.
   - `flags` is a map of flag names to bools that indicate which flags have been set for the specified resource by the bridge authority.
   - `params` (optional) is a map of parameter names to values that must be set by the client to use this transport.
- `full_update` is a bool that indicates whether the backend has finished sending all udpated information. If this is false, another resource diff will immediately follow.

<details>
<summary>Example:</summary>

```
{
    "new": {
        "obfs2": [
            {
                "type": "obfs2",
                "blocked_in": {},
                "Location": null,
                "protocol": "tcp",
                "address": "176.247.216.207",
                "port": 42810,
                "fingerprint": "10282810115283F99ADE5CFE42D49644F45D715D",
                "or-addresses": null,
                "distribution": "https",
                "flags": {
                    "fast": true,
                    "stable": true,
                    "running": true,
                    "valid": true
                }
            },
            {
                "type": "obfs2",
                "blocked_in": {},
                "Location": null,
                "protocol": "tcp",
                "address": "133.69.16.145",
                "port": 58314,
                "fingerprint": "BE84A97D02130470A1C77839954392BA979F7EE1",
                "or-addresses": null,
                "distribution": "https",
                "flags": {
                    "fast": true,
                    "stable": true,
                    "running": true,
                    "valid": true
                }
            }
        ],
        "scramblesuit": [
            {
                "type": "scramblesuit",
                "blocked_in": {},
                "Location": null,
                "protocol": "tcp",
                "address": "216.117.3.62",
                "port": 63174,
                "fingerprint": "BE84A97D02130470A1C77839954392BA979F7EE1",
                "or-addresses": null,
                "distribution": "https",
                "flags": {
                    "fast": true,
                    "stable": true,
                    "running": true,
                    "valid": true
                },
                "params": {
                    "password": "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
                }
            }
        ]
    },
    "changed": null,
    "gone": null,
    "full_update": true
}
```

</details>


### Making a static request for resources

Distributors also have the option to make a `GET` request to the `resources` endpoint for a full static list of all resources available for the given distribution method. As opposed to the resource stream, the HTTP response body will contain only a single resource diff

`GET /resources HTTP/1.1`

##### Headers
- `Host:` must be set
- `Authorization: Bearer [token]` must be set to the API bearer token
- `Content-Length:` must be set to the length of the supplied data for GET requests

##### Data

Distributors must send a JSON object with the following data:
```
{
  "request_origin": string,
  "resouce_types": [string]
}
```
where:
- `request_origin` is a string with the name of the distributor. This must correspond to a known distributor, specified in the config file for the rdsys backend.
- `resource_types` is a list of strings of requested resource types (e.g., "vanilla", "obfs4", "snowflake", etc.). Unknown resource types will be ignored.

<details>
<summary>Example:</summary>

```
GET /resources HTTP/1.1
Host: localhost:7100
Authorization: Bearer HttpsApiTokenPlaceholder
Content-Type: application/json
Content-Length: 68

{"request_origin":"https","resource_types":["obfs2","scramblesuit"]}
```

</details>


### Response 
The HTTP response to the `GET` resources API call is a chunked transfer encoding of a list of JSON objects that represent all the resources currently allocated to the distributor.

```
[
  Resource,
  Resource,
  ...
  Resource
]

```

where `Resource` is specified above.


<details>
<summary>Example:</summary>

```
[
    {
        "type": "obfs2",
        "blocked_in": {},
        "Location": null,
        "protocol": "tcp",
        "address": "176.247.216.207",
        "port": 42810,
        "fingerprint": "10282810115283F99ADE5CFE42D49644F45D715D",
        "or-addresses": null,
        "distribution": "https",
        "flags": {
            "fast": true,
            "stable": true,
            "running": true,
            "valid": true
        }
    },
    {
        "type": "obfs2",
        "blocked_in": {},
        "Location": null,
        "protocol": "tcp",
        "address": "133.69.16.145",
        "port": 58314,
        "fingerprint": "BE84A97D02130470A1C77839954392BA979F7EE1",
        "or-addresses": null,
        "distribution": "https",
        "flags": {
            "fast": true,
            "stable": true,
            "running": true,
            "valid": true
        }
    },
    {
        "type": "scramblesuit",
        "blocked_in": {},
        "Location": null,
        "protocol": "tcp",
        "address": "216.117.3.62",
        "port": 63174,
        "fingerprint": "BE84A97D02130470A1C77839954392BA979F7EE1",
        "or-addresses": null,
        "distribution": "https",
        "flags": {
            "fast": true,
            "stable": true,
            "running": true,
            "valid": true
        },
        "params": {
            "password": "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
        }
    }
]
```

</details>
