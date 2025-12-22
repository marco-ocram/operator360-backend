# Opt360 Portal Backend - API Documentation

## Base URL
```
http://localhost:8080/api
```

## Authentication
All API endpoints require authentication via the `AuthMiddleware`. Include the appropriate authentication headers with each request.

---

## Endpoints

### 1. Get User Information
Get the current authenticated user's information and regional office.

**Endpoint:** `GET /api/user/info`

**Authentication:** Required

**Response:**
```json
{
  "ad_id": "john.doe",
  "regional_office": "Pune"
}
```

**Status Codes:**
- `200 OK` - Success
- `401 Unauthorized` - User not authenticated

---

### 2. Get KPI Data
Fetches KPI data from `kpi.json` file based on the user's regional office.

**Endpoint:** `GET /api/kpi`

**Authentication:** Required

**Response:**
```json
{
  "kpi_data": "...",
  "metrics": "..."
}
```

**Status Codes:**
- `200 OK` - Success
- `401 Unauthorized` - User not authenticated
- `404 Not Found` - KPI file not found for regional office
- `500 Internal Server Error` - S3 client error or file read error

---

### 3. Get RO Risk Distribution
Fetches regional office risk distribution data from `opt360Store/kpi.json`.

**Endpoint:** `GET /api/ro_risk_distribution`

**Authentication:** Required

**Response:**
```json
{
  "file": "opt360Store/kpi.json",
  "requested_by": "john.doe",
  "regional_office": "Pune",
  "data": {
    "risk_distribution": "..."
  }
}
```

**Status Codes:**
- `200 OK` - Success
- `401 Unauthorized` - User not authenticated
- `404 Not Found` - Risk distribution file not found
- `500 Internal Server Error` - S3 client error or JSON parsing error

---

### 4. Get Operator List
Fetches the list of operators with optional filtering capabilities.

**Endpoint:** `GET /api/operator_list`

**Authentication:** Required

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `opt_ea` | string | No | Filter by EA (exact match, case-insensitive) |
| `opt_reg` | string | No | Filter by region (exact match, case-insensitive) |
| `opt_district` | string | No | Filter by district (exact match, case-insensitive) |
| `opt_state` | string | No | Filter by state (exact match, case-insensitive) |
| `opt_id` | string | No | Search by operator ID (partial match, case-insensitive) |
| `page` | integer | No | Page number (default: 1) |
| `page_size` | integer | No | Records per page (default: 20, max: 1000) |

**Example Request:**
```
GET /api/operator_list?opt_state=Maharashtra&opt_district=Pune&page=1&page_size=50
```

**Response:**
```json
{
  "regional_office": "Pune",
  "file": "opt360Store/Pune/operator.parquet",
  "total_count": 1500,
  "filtered_count": 150,
  "pagination": {
    "page": 1,
    "page_size": 50,
    "total_records": 150,
    "total_pages": 3,
    "has_next": true,
    "has_previous": false
  },
  "filters": {
    "opt_ea": "",
    "opt_reg": "",
    "opt_district": "Pune",
    "opt_state": "Maharashtra",
    "opt_id": ""
  },
  "count": 50,
  "data": [
    {
      "opt_id": "MH_DOP_PN_NS123456",
      "opt_state": "Maharashtra",
      "opt_district": "Pune",
      "..."
    }
  ]
}
```

**Status Codes:**
- `200 OK` - Success
- `401 Unauthorized` - User not authenticated
- `404 Not Found` - Operator list file not found
- `500 Internal Server Error` - S3 client error or parquet read error

---

### 5. Get High Risk Operators
Fetches the list of high-risk operators from `operator_high.parquet`.

**Endpoint:** `GET /api/high_risk_operator`

**Authentication:** Required

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `page` | integer | No | Page number (default: 1) |
| `page_size` | integer | No | Records per page (default: 20, max: 1000) |

**Example Request:**
```
GET /api/high_risk_operator?page=1&page_size=50
```

**Response:**
```json
{
  "regional_office": "Pune",
  "file": "opt360Store/Pune/operator_high.parquet",
  "pagination": {
    "page": 1,
    "page_size": 50,
    "total_records": 245,
    "total_pages": 5,
    "has_next": true,
    "has_previous": false
  },
  "count": 50,
  "data": [
    {
      "opt_id": "MH_DOP_PN_NS123456",
      "risk_score": 0.95,
      "..."
    }
  ]
}
```

**Status Codes:**
- `200 OK` - Success
- `401 Unauthorized` - User not authenticated
- `404 Not Found` - High risk operators file not found
- `500 Internal Server Error` - S3 client error or parquet read error

---

### 6. Get Medium Risk Operators
Fetches the list of medium-risk operators from `operator_medium.parquet`.

**Endpoint:** `GET /api/med_risk_operator`

**Authentication:** Required

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `page` | integer | No | Page number (default: 1) |
| `page_size` | integer | No | Records per page (default: 20, max: 1000) |

**Example Request:**
```
GET /api/med_risk_operator?page=1&page_size=50
```

**Response:**
```json
{
  "regional_office": "Pune",
  "file": "opt360Store/Pune/operator_medium.parquet",
  "pagination": {
    "page": 1,
    "page_size": 50,
    "total_records": 568,
    "total_pages": 12,
    "has_next": true,
    "has_previous": false
  },
  "count": 50,
  "data": [
    {
      "opt_id": "MH_DOP_PN_NS123456",
      "risk_score": 0.65,
      "..."
    }
  ]
}
```

**Status Codes:**
- `200 OK` - Success
- `401 Unauthorized` - User not authenticated
- `404 Not Found` - Medium risk operators file not found
- `500 Internal Server Error` - S3 client error or parquet read error

---

### 7. Get Low Risk Operators
Fetches the list of low-risk operators from `operator_low.parquet`.

**Endpoint:** `GET /api/low_risk_operator`

**Authentication:** Required

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `page` | integer | No | Page number (default: 1) |
| `page_size` | integer | No | Records per page (default: 20, max: 1000) |

**Example Request:**
```
GET /api/low_risk_operator?page=1&page_size=50
```

**Response:**
```json
{
  "regional_office": "Pune",
  "file": "opt360Store/Pune/operator_low.parquet",
  "pagination": {
    "page": 1,
    "page_size": 50,
    "total_records": 1024,
    "total_pages": 21,
    "has_next": true,
    "has_previous": false
  },
  "count": 50,
  "data": [
    {
      "opt_id": "MH_DOP_PN_NS123456",
      "risk_score": 0.25,
      "..."
    }
  ]
}
```

**Status Codes:**
- `200 OK` - Success
- `401 Unauthorized` - User not authenticated
- `404 Not Found` - Low risk operators file not found
- `500 Internal Server Error` - S3 client error or parquet read error

---

### 8. Get Operator Details
Fetches detailed information about a specific operator from `opt_details.parquet`.

**Endpoint:** `GET /api/operator_details`

**Authentication:** Required

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `opt_state` | string | Yes | Operator's state (e.g., "Maharashtra") |
| `opt_district` | string | Yes | Operator's district (e.g., "Pune") |
| `opt_id` | string | Yes | Operator ID (e.g., "MH_DOP_PN_NS123456") |
| `page` | integer | No | Page number (default: 1) |
| `page_size` | integer | No | Records per page (default: 20, max: 1000) |

**Example Request:**
```
GET /api/operator_details?opt_state=Maharashtra&opt_district=Pune&opt_id=MH_DOP_PN_NS123456
```

**Response:**
```json
{
  "regional_office": "Pune",
  "operator_id": "MH_DOP_PN_NS123456",
  "state": "Maharashtra",
  "district": "Pune",
  "file": "opt360Store/Pune/Maharashtra/Pune/MH_DOP_PN_NS123456/opt_details.parquet",
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total_records": 45,
    "total_pages": 3,
    "has_next": true,
    "has_previous": false
  },
  "count": 20,
  "data": [
    {
      "detail_field_1": "value1",
      "detail_field_2": "value2",
      "..."
    }
  ],
  "requested_by": "john.doe"
}
```

**Status Codes:**
- `200 OK` - Success
- `400 Bad Request` - Missing required query parameters
- `401 Unauthorized` - User not authenticated
- `404 Not Found` - Operator details file not found
- `500 Internal Server Error` - S3 client error or parquet read error

**Notes:**
- Spaces in state, district, or operator ID are automatically converted to underscores for S3 path compatibility
- Example: "Mumbai City" → "Mumbai_City"

---

### 9. Get Operator Packets
Fetches the latest date's packet files (SID files) for a specific operator.

**Endpoint:** `GET /api/operator_packets`

**Authentication:** Required

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `opt_state` | string | Yes | Operator's state (e.g., "Gujarat") |
| `opt_district` | string | Yes | Operator's district (e.g., "Vadodara") |
| `opt_id` | string | Yes | Operator ID (e.g., "GJ_DOP_VDR_NS764416") |
| `page` | integer | No | Page number (default: 1) |
| `page_size` | integer | No | Records per page (default: 10, max: 1000) |

**Example Request:**
```
GET /api/operator_packets?opt_state=Gujarat&opt_district=Vadodara&opt_id=GJ_DOP_VDR_NS764416
```

**Response:**
```json
{
  "regional_office": "Pune",
  "operator_id": "GJ_DOP_VDR_NS764416",
  "state": "Gujarat",
  "district": "Vadodara",
  "folder_path": "opt360Store/Pune/Gujarat/Vadodara/GJ_DOP_VDR_NS764416/",
  "latest_date": "2025_12_04",
  "files_found": 2,
  "files_processed": [
    {
      "file_name": "sid_2025_12_04_U.parquet",
      "file_path": "opt360Store/Pune/Gujarat/Vadodara/GJ_DOP_VDR_NS764416/sid_2025_12_04_U.parquet",
      "date": "2025_12_04",
      "packet_type": "update",
      "record_count": 150,
      "status": "success"
    },
    {
      "file_name": "sid_2025_12_04_N.parquet",
      "file_path": "opt360Store/Pune/Gujarat/Vadodara/GJ_DOP_VDR_NS764416/sid_2025_12_04_N.parquet",
      "date": "2025_12_04",
      "packet_type": "new_enrollment",
      "record_count": 75,
      "status": "success"
    }
  ],
  "total_records": 225,
  "pagination": {
    "page": 1,
    "page_size": 10,
    "total_records": 225,
    "total_pages": 23,
    "has_next": true,
    "has_previous": false
  },
  "count": 10,
  "data": [
    {
      "eid": "123456789012",
      "source_file": "sid_2025_12_04_U.parquet",
      "packet_type": "update",
      "date": "2025_12_04",
      "..."
    }
  ],
  "requested_by": "john.doe"
}
```

**Status Codes:**
- `200 OK` - Success
- `400 Bad Request` - Missing required query parameters
- `401 Unauthorized` - User not authenticated
- `404 Not Found` - No packet files found for the operator
- `500 Internal Server Error` - S3 client error or parquet read error

**Notes:**
- Automatically identifies the latest date from available packet files
- Combines data from both update packets (_U) and new enrollment packets (_N)
- Each record includes metadata: `source_file`, `packet_type`, and `date`

---

### 10. Search Operator Packets by SID
Searches for specific enrollments in packet files for a given date with advanced filtering.

**Endpoint:** `GET /api/search_operator_packets`

**Authentication:** Required

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `opt_state` | string | Yes | Operator's state (e.g., "Gujarat") |
| `opt_district` | string | Yes | Operator's district (e.g., "Vadodara") |
| `opt_id` | string | Yes | Operator ID (e.g., "GJ_DOP_VDR_NS764416") |
| `date` | string | Yes | Date in format YYYY_MM_DD (e.g., "2025_12_12") |
| `sid` | string | No | Search by SID/EID (partial match, case-insensitive) |
| `anomaly_filter` | string | No | Filter by anomaly status: "anomalous" or "non-anomalous" |
| `enrollment_type` | string | No | Filter by enrollment type: "update" or "new_enrollment" |
| `page` | integer | No | Page number (default: 1) |
| `page_size` | integer | No | Records per page (default: 50, max: 1000) |

**Example Requests:**
```
# Get all packets for a specific date
GET /api/search_operator_packets?opt_state=Gujarat&opt_district=Vadodara&opt_id=GJ_DOP_VDR_NS764416&date=2025_12_12

# Search for a specific SID
GET /api/search_operator_packets?opt_state=Gujarat&opt_district=Vadodara&opt_id=GJ_DOP_VDR_NS764416&date=2025_12_12&sid=123456789012

# Get only anomalous update packets
GET /api/search_operator_packets?opt_state=Gujarat&opt_district=Vadodara&opt_id=GJ_DOP_VDR_NS764416&date=2025_12_12&anomaly_filter=anomalous&enrollment_type=update
```

**Response:**
```json
{
  "regional_office": "Pune",
  "operator_id": "GJ_DOP_VDR_NS764416",
  "state": "Gujarat",
  "district": "Vadodara",
  "folder_path": "opt360Store/Pune/Gujarat/Vadodara/GJ_DOP_VDR_NS764416/",
  "selected_date": "2025_12_12",
  "search_sid": "123456789012",
  "anomaly_filter": "anomalous",
  "enrollment_type_filter": "update",
  "files_processed": [
    {
      "file_name": "all_sid_2025_12_12_U.parquet",
      "packet_type": "update",
      "record_count": 5,
      "status": "success"
    },
    {
      "file_name": "all_sid_2025_12_12_N.parquet",
      "packet_type": "new_enrollment",
      "status": "not_found",
      "error": "..."
    }
  ],
  "total_records": 5,
  "pagination": {
    "page": 1,
    "page_size": 50,
    "total_records": 5,
    "total_pages": 1,
    "has_next": false,
    "has_previous": false
  },
  "count": 5,
  "data": [
    {
      "eid": "123456789012",
      "anomaly_type": ["type1", "type2"],
      "enrolnment_type": "U",
      "packet_type": "update",
      "date": "2025_12_12",
      "..."
    }
  ],
  "requested_by": "john.doe"
}
```

**Status Codes:**
- `200 OK` - Success (even if only one file type is found)
- `400 Bad Request` - Missing required query parameters
- `401 Unauthorized` - User not authenticated
- `404 Not Found` - No packet files found for the selected date
- `500 Internal Server Error` - S3 client error or parquet read error

**Filter Details:**

**SID Filter (`sid`):**
- Performs case-insensitive partial matching on the `eid` field
- Returns all records containing the search term

**Anomaly Filter (`anomaly_filter`):**
- `"anomalous"` - Returns only records with non-empty `anomaly_type` field
- `"non-anomalous"` - Returns only records with empty `anomaly_type` field

**Enrollment Type Filter (`enrollment_type`):**
- `"update"` - Returns only records with `enrolnment_type` = "U"
- `"new_enrollment"` - Returns only records with `enrolnment_type` = "N"

**Notes:**
- All filters can be combined for advanced queries
- Files are named: `all_sid_YYYY_MM_DD_U.parquet` (updates) and `all_sid_YYYY_MM_DD_N.parquet` (new enrollments)
- The API attempts to read both file types and returns available data

---

### 11. Submit Feedback
Submits feedback data as a JSON file to S3 storage.

**Endpoint:** `POST /api/feedback`

**Authentication:** Required

**Request Body:**
```json
{
  "opt_state": "Maharashtra",
  "opt_district": "Mumbai City",
  "opt_id": "MH_DOP_MC_NO047414",
  "submitted_by": "john.doe",
  "timestamp": "2025-12-16T10:30:00.000Z",
  "feedback": {
    "verified_fraud_operator": {
      "verdict": true,
      "remarks": "Confirmed fraudulent activity detected"
    },
    "additional_comments": "Requires immediate action"
  }
}
```

**Required Fields:**
- `opt_state` (string) - Operator's state
- `opt_district` (string) - Operator's district
- `opt_id` (string) - Operator ID

**Optional Fields:**
- Any additional JSON fields can be included in the request body

**Response:**
```json
{
  "message": "Feedback submitted successfully",
  "regional_office": "Pune",
  "operator_id": "MH_DOP_MC_NO047414",
  "state": "Maharashtra",
  "district": "Mumbai City",
  "file_path": "opt360Store/Pune/Maharashtra/Mumbai_City/MH_DOP_MC_NO047414/2025_12_16_john.doe.json",
  "date": "2025_12_16",
  "submitted_by": "john.doe"
}
```

**Status Codes:**
- `200 OK` - Feedback submitted successfully
- `400 Bad Request` - Invalid JSON or missing required fields
- `401 Unauthorized` - User not authenticated
- `500 Internal Server Error` - S3 client error or upload failure

**Notes:**
- File naming convention: `YYYY_MM_DD_{ad-id}.json`
- Spaces in state, district, or operator ID are converted to underscores for S3 path
- The entire request body is saved as a JSON file in S3
- Files are stored in: `opt360Store/{RegionalOffice}/{State}/{District}/{OperatorID}/`

---

### 12. Get Anomaly Indicators
Fetches anomaly insights data for the regional office.

**Endpoint:** `GET /api/anamoly_indicators`

**Authentication:** Required

**Response:**
```json
{
  "file": "opt360Store/Pune/anomalies_insights.json",
  "requested_by": "john.doe",
  "regional_office": "Pune",
  "data": {
    "anomaly_indicators": [
      {
        "type": "fraud_pattern_1",
        "description": "...",
        "severity": "high"
      }
    ],
    "insights": "..."
  }
}
```

**Status Codes:**
- `200 OK` - Success
- `401 Unauthorized` - User not authenticated
- `404 Not Found` - Anomaly indicators file not found for regional office
- `500 Internal Server Error` - S3 client error or JSON parsing error

**Notes:**
- Automatically handles Python-style JSON (single quotes converted to double quotes)
- File location: `opt360Store/{RegionalOffice}/anomalies_insights.json`

---

## Common Response Fields

### Pagination Object
All paginated endpoints return a `pagination` object with the following structure:
```json
{
  "pagination": {
    "page": 1,
    "page_size": 50,
    "total_records": 245,
    "total_pages": 5,
    "has_next": true,
    "has_previous": false
  }
}
```

### Error Response
All error responses follow this structure:
```json
{
  "error": "Error message",
  "details": "Detailed error information"
}
```

---

## S3 File Structure

```
opt360Store/
├── kpi.json                                    # Global risk distribution
├── {RegionalOffice}/
│   ├── kpi.json                               # Regional KPI data
│   ├── operator.parquet                       # All operators list
│   ├── operator_high.parquet                  # High risk operators
│   ├── operator_medium.parquet                # Medium risk operators
│   ├── operator_low.parquet                   # Low risk operators
│   ├── anomalies_insights.json                # Anomaly indicators
│   └── {State}/
│       └── {District}/
│           └── {OperatorID}/
│               ├── opt_details.parquet        # Operator details
│               ├── sid_YYYY_MM_DD_U.parquet   # Update packets
│               ├── sid_YYYY_MM_DD_N.parquet   # New enrollment packets
│               ├── all_sid_YYYY_MM_DD_U.parquet   # All update packets
│               ├── all_sid_YYYY_MM_DD_N.parquet   # All new enrollment packets
│               └── YYYY_MM_DD_{ad-id}.json    # Feedback files
```

---

## Notes

1. **Path Sanitization:** Spaces in state, district, and operator ID parameters are automatically converted to underscores for S3 path compatibility.

2. **Regional Office Context:** Most endpoints automatically filter data based on the authenticated user's regional office.

3. **Pagination:** Default pagination is applied to prevent large data transfers. Adjust `page_size` as needed (max: 1000).

4. **File Formats:**
   - `.parquet` - Columnar data format for efficient storage and retrieval
   - `.json` - Standard JSON format for configuration and metadata

5. **Date Format:** All dates use the format `YYYY_MM_DD` (e.g., "2025_12_16").

6. **Packet Types:**
   - `U` or `update` - Update packets for existing enrollments
   - `N` or `new_enrollment` - New enrollment packets

7. **Case Sensitivity:** All text-based filters are case-insensitive for better user experience.

---

## Configuration

The API uses `config.json` for S3 configuration:

```json
{
  "server": {
    "host": "localhost",
    "port": 8080
  },
  "s3": {
    "bucket_name": "prd-dsw-bronze-0",
    "access_key": "...",
    "secret_key": "...",
    "endpoint": "http://10.10.103.14:423",
    "region": "us-east-1"
  }
}
```

---

## Version
API Version: 1.0  
Last Updated: December 20, 2025
