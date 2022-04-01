/*
 * NSX-T Manager API
 *
 * VMware NSX-T Manager REST API
 *
 * API version: 3.2.0.0.0
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package supportbundle

type FailedNodeSupportBundleResult struct {
	// Error code
	ErrorCode string `json:"error_code,omitempty"`
	// Error message
	ErrorMessage string `json:"error_message,omitempty"`
	// Display name of node
	NodeDisplayName string `json:"node_display_name,omitempty"`
	// UUID of node
	NodeId string `json:"node_id,omitempty"`
}
