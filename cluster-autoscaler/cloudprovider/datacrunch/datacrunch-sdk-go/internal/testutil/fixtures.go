package testutil

import "time"

// MockAPIResponses contains common API response fixtures
var MockAPIResponses = struct {
	SuccessResponse map[string]interface{}
	ErrorResponse   map[string]interface{}
	InstanceList    map[string]interface{}
	SSHKeyList      map[string]interface{}
}{
	SuccessResponse: map[string]interface{}{
		"status":  "success",
		"message": "Operation completed successfully",
	},
	ErrorResponse: map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "INVALID_REQUEST",
			"message": "The request is invalid",
			"details": "Missing required field: instance_type",
		},
	},
	InstanceList: map[string]interface{}{
		"instances": []map[string]interface{}{
			{
				"id":            "inst-123456",
				"name":          "test-instance-1",
				"status":        "running",
				"instance_type": "v1.small",
				"location":      "us-east-1",
				"created_at":    time.Now().Format(time.RFC3339),
			},
			{
				"id":            "inst-789012",
				"name":          "test-instance-2",
				"status":        "stopped",
				"instance_type": "v1.medium",
				"location":      "us-west-2",
				"created_at":    time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			},
		},
		"total": 2,
	},
	SSHKeyList: map[string]interface{}{
		"ssh_keys": []map[string]interface{}{
			{
				"id":          "key-123456",
				"name":        "my-key",
				"fingerprint": "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
				"created_at":  time.Now().Format(time.RFC3339),
			},
		},
		"total": 1,
	},
}

// MockHTTPErrors contains common HTTP error scenarios
var MockHTTPErrors = struct {
	BadRequest          int
	Unauthorized        int
	Forbidden           int
	NotFound            int
	MethodNotAllowed    int
	InternalServerError int
	BadGateway          int
	ServiceUnavailable  int
	GatewayTimeout      int
}{
	BadRequest:          400,
	Unauthorized:        401,
	Forbidden:           403,
	NotFound:            404,
	MethodNotAllowed:    405,
	InternalServerError: 500,
	BadGateway:          502,
	ServiceUnavailable:  503,
	GatewayTimeout:      504,
}

// MockRequestBodies contains sample request bodies for testing
var MockRequestBodies = struct {
	CreateInstance map[string]interface{}
	CreateSSHKey   map[string]interface{}
}{
	CreateInstance: map[string]interface{}{
		"instance_type": "v1.small",
		"image":         "ubuntu-20.04",
		"ssh_key_ids":   []string{"key-123456"},
		"hostname":      "test-instance",
		"description":   "Test instance for SDK",
		"location_code": "us-east-1",
		"os_volume": map[string]interface{}{
			"name": "root",
			"size": 50,
		},
		"is_spot":  false,
		"contract": "hourly",
		"pricing":  "standard",
	},
	CreateSSHKey: map[string]interface{}{
		"name":       "test-key",
		"public_key": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC...",
	},
}

// GetMockErrorResponse returns a mock error response with the given status code
func GetMockErrorResponse(statusCode int, message string) map[string]interface{} {
	return map[string]interface{}{
		"error": map[string]interface{}{
			"code":    statusCode,
			"message": message,
		},
	}
}

// GetMockListResponse returns a mock list response with pagination
func GetMockListResponse(items []interface{}, total int) map[string]interface{} {
	return map[string]interface{}{
		"data":  items,
		"total": total,
		"page":  1,
		"limit": len(items),
	}
}
