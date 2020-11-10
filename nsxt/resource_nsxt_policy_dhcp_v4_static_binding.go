/* Copyright © 2019 VMware, Inc. All Rights Reserved.
   SPDX-License-Identifier: MPL-2.0 */

package nsxt

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/bindings"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/data"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/protocol/client"
	gm_segments "github.com/vmware/vsphere-automation-sdk-go/services/nsxt-gm/global_infra/segments"
	gm_model "github.com/vmware/vsphere-automation-sdk-go/services/nsxt-gm/model"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/infra/segments"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"
)

func resourceNsxtPolicyDhcpV4StaticBinding() *schema.Resource {
	return &schema.Resource{
		Create: resourceNsxtPolicyDhcpV4StaticBindingCreate,
		Read:   resourceNsxtPolicyDhcpV4StaticBindingRead,
		Update: resourceNsxtPolicyDhcpV4StaticBindingUpdate,
		Delete: resourceNsxtPolicyDhcpV4StaticBindingDelete,
		Importer: &schema.ResourceImporter{
			State: nsxtSegmentResourceImporter,
		},

		Schema: map[string]*schema.Schema{
			"nsx_id":       getNsxIDSchema(),
			"path":         getPathSchema(),
			"display_name": getDisplayNameSchema(),
			"description":  getDescriptionSchema(),
			"revision":     getRevisionSchema(),
			"tag":          getTagsSchema(),
			"segment_path": getPolicyPathSchema(true, true, "segment path"),
			"gateway_address": {
				Type:         schema.TypeString,
				Description:  "When not specified, gateway address is auto-assigned from segment configuration",
				ValidateFunc: validation.IsIPv4Address,
				Optional:     true,
			},
			"hostname": {
				Type:        schema.TypeString,
				Description: "Hostname to assign to the host",
				Optional:    true,
			},
			"ip_address": {
				Type:         schema.TypeString,
				Description:  "IP assigned to host. The IP address must belong to the subnetconfigured on segment",
				ValidateFunc: validation.IsIPv4Address,
				Required:     true,
			},
			"lease_time": getDhcpLeaseTimeSchema(),
			"mac_address": {
				Type:         schema.TypeString,
				Description:  "MAC address of the host",
				Required:     true,
				ValidateFunc: validation.IsMACAddress,
			},
			"dhcp_option_121":     getDhcpOptions121Schema(),
			"dhcp_generic_option": getDhcpGenericOptionsSchema(),
		},
	}
}

func getPolicyDchpStaticBindingOnSegment(id string, segmentID string, connector *client.RestConnector, isGlobalManager bool) (*data.StructValue, error) {
	if isGlobalManager {
		client := gm_segments.NewDefaultDhcpStaticBindingConfigsClient(connector)
		return client.Get(segmentID, id)
	}
	client := segments.NewDefaultDhcpStaticBindingConfigsClient(connector)
	return client.Get(segmentID, id)
}

func resourceNsxtPolicyDhcpStaticBindingExistsOnSegment(id string, segmentID string, connector *client.RestConnector, isGlobalManager bool) (bool, error) {
	_, err := getPolicyDchpStaticBindingOnSegment(id, segmentID, connector, isGlobalManager)
	if err == nil {
		return true, nil
	}

	if isNotFoundError(err) {
		return false, nil
	}

	return false, logAPIError("Error retrieving resource", err)
}

func resourceNsxtPolicyDhcpStaticBindingExists(segmentID string) func(id string, connector *client.RestConnector, isGlobalManager bool) (bool, error) {
	return func(id string, connector *client.RestConnector, isGlobalManager bool) (bool, error) {
		return resourceNsxtPolicyDhcpStaticBindingExistsOnSegment(id, segmentID, connector, isGlobalManager)
	}
}

func policyDhcpV4StaticBindingConvertAndPatch(d *schema.ResourceData, segmentID string, id string, m interface{}) error {

	displayName := d.Get("display_name").(string)
	description := d.Get("description").(string)
	tags := getPolicyTagsFromSchema(d)
	gatewayAddress := d.Get("gateway_address").(string)
	hostName := d.Get("hostname").(string)
	ipAddress := d.Get("ip_address").(string)
	leaseTime := int64(d.Get("lease_time").(int))
	macAddress := d.Get("mac_address").(string)
	dhcpOptions := getDhcpOptsFromSchema(d)

	obj := model.DhcpV4StaticBindingConfig{
		DisplayName:  &displayName,
		Description:  &description,
		Tags:         tags,
		HostName:     &hostName,
		IpAddress:    &ipAddress,
		LeaseTime:    &leaseTime,
		MacAddress:   &macAddress,
		Options:      dhcpOptions,
		ResourceType: "DhcpV4StaticBindingConfig",
	}

	if len(gatewayAddress) > 0 {
		obj.GatewayAddress = &gatewayAddress
	}

	connector := getPolicyConnector(m)

	converter := bindings.NewTypeConverter()
	converter.SetMode(bindings.REST)

	if isPolicyGlobalManager(m) {
		convObj, convErrs := converter.ConvertToVapi(obj, gm_model.DhcpV4StaticBindingConfigBindingType())
		if convErrs != nil {
			return convErrs[0]
		}
		client := gm_segments.NewDefaultDhcpStaticBindingConfigsClient(connector)
		return client.Patch(segmentID, id, convObj.(*data.StructValue))
	}
	convObj, convErrs := converter.ConvertToVapi(obj, model.DhcpV4StaticBindingConfigBindingType())
	if convErrs != nil {
		return convErrs[0]
	}
	client := segments.NewDefaultDhcpStaticBindingConfigsClient(connector)
	return client.Patch(segmentID, id, convObj.(*data.StructValue))
}

func getDhcpOptsFromSchema(d *schema.ResourceData) *model.DhcpV4Options {
	dhcpOpts := model.DhcpV4Options{}

	dhcp121Opts := d.Get("dhcp_option_121").([]interface{})
	if len(dhcp121Opts) > 0 {
		dhcp121OptStruct := getPolicyDhcpOptions121(dhcp121Opts)
		dhcpOpts.Option121 = &dhcp121OptStruct
	}

	otherDhcpOpts := d.Get("dhcp_generic_option").([]interface{})
	if len(otherDhcpOpts) > 0 {
		otherOptStructs := getPolicyDhcpGenericOptions(otherDhcpOpts)
		dhcpOpts.Others = otherOptStructs
	}

	if len(dhcp121Opts)+len(otherDhcpOpts) > 0 {
		return &dhcpOpts
	}

	return nil

}

func resourceNsxtPolicyDhcpV4StaticBindingCreate(d *schema.ResourceData, m interface{}) error {

	segmentPath := d.Get("segment_path").(string)
	segmentID := getPolicyIDFromPath(segmentPath)
	// Initialize resource Id and verify this ID is not yet used
	id, err := getOrGenerateID(d, m, resourceNsxtPolicyDhcpStaticBindingExists(segmentID))
	if err != nil {
		return err
	}

	log.Printf("[INFO] Creating DhcpV4StaticBindingConfig with ID %s", id)
	err = policyDhcpV4StaticBindingConvertAndPatch(d, segmentID, id, m)

	if err != nil {
		return handleCreateError("DhcpV4StaticBindingConfig", id, err)
	}

	d.SetId(id)
	d.Set("nsx_id", id)

	return resourceNsxtPolicyDhcpV4StaticBindingRead(d, m)
}

func resourceNsxtPolicyDhcpV4StaticBindingRead(d *schema.ResourceData, m interface{}) error {
	connector := getPolicyConnector(m)

	id := d.Id()
	if id == "" {
		return fmt.Errorf("Error obtaining DhcpV4StaticBindingConfig ID")
	}

	segmentPath := d.Get("segment_path").(string)
	segmentID := getPolicyIDFromPath(segmentPath)

	var obj model.DhcpV4StaticBindingConfig
	converter := bindings.NewTypeConverter()
	converter.SetMode(bindings.REST)
	var err error
	var dhcpObj *data.StructValue
	if isPolicyGlobalManager(m) {
		client := gm_segments.NewDefaultDhcpStaticBindingConfigsClient(connector)
		dhcpObj, err = client.Get(segmentID, id)

	} else {
		client := segments.NewDefaultDhcpStaticBindingConfigsClient(connector)
		dhcpObj, err = client.Get(segmentID, id)
	}
	if err != nil {
		return handleReadError(d, "DhcpV4StaticBindingConfig", id, err)
	}

	convObj, errs := converter.ConvertToGolang(dhcpObj, model.DhcpV4StaticBindingConfigBindingType())
	if errs != nil {
		return errs[0]
	}
	obj = convObj.(model.DhcpV4StaticBindingConfig)

	if obj.ResourceType != "DhcpV4StaticBindingConfig" {
		return handleReadError(d, "DhcpV4StaticBindingConfig", id, fmt.Errorf("Unexpected ResourceType"))
	}

	d.Set("display_name", obj.DisplayName)
	d.Set("description", obj.Description)
	setPolicyTagsInSchema(d, obj.Tags)
	d.Set("nsx_id", id)
	d.Set("path", obj.Path)
	d.Set("revision", obj.Revision)

	d.Set("gateway_address", obj.GatewayAddress)
	d.Set("hostname", obj.HostName)
	d.Set("ip_address", obj.IpAddress)
	d.Set("lease_time", obj.LeaseTime)
	d.Set("mac_address", obj.MacAddress)

	if obj.Options != nil {
		if obj.Options.Option121 != nil {
			d.Set("dhcp_option_121", getPolicyDhcpOptions121FromStruct(obj.Options.Option121))
		}

		if len(obj.Options.Others) > 0 {
			d.Set("dhcp_generic_option", getPolicyDhcpGenericOptionsFromStruct(obj.Options.Others))
		}
	}

	return nil
}

func resourceNsxtPolicyDhcpV4StaticBindingUpdate(d *schema.ResourceData, m interface{}) error {
	id := d.Id()
	if id == "" {
		return fmt.Errorf("Error obtaining DhcpV4StaticBindingConfig ID")
	}
	segmentPath := d.Get("segment_path").(string)
	segmentID := getPolicyIDFromPath(segmentPath)

	log.Printf("[INFO] Updating DhcpV4StaticBindingConfig with ID %s", id)
	err := policyDhcpV4StaticBindingConvertAndPatch(d, segmentID, id, m)
	if err != nil {
		return handleUpdateError("DhcpV4StaticBindingConfig", id, err)
	}

	return resourceNsxtPolicyDhcpV4StaticBindingRead(d, m)
}

func resourceNsxtPolicyDhcpV4StaticBindingDelete(d *schema.ResourceData, m interface{}) error {
	id := d.Id()
	if id == "" {
		return fmt.Errorf("Error obtaining DhcpV4StaticBindingConfig ID")
	}
	segmentPath := d.Get("segment_path").(string)
	segmentID := getPolicyIDFromPath(segmentPath)

	connector := getPolicyConnector(m)
	var err error
	if isPolicyGlobalManager(m) {
		client := gm_segments.NewDefaultDhcpStaticBindingConfigsClient(connector)
		err = client.Delete(segmentID, id)
	} else {
		client := segments.NewDefaultDhcpStaticBindingConfigsClient(connector)
		err = client.Delete(segmentID, id)
	}

	if err != nil {
		return handleDeleteError("DhcpV4StaticBindingConfig", id, err)
	}

	return nil
}

func nsxtSegmentResourceImporter(d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	importID := d.Id()
	importSegment := ""
	importGW := ""
	s := strings.Split(importID, "/")
	if len(s) < 2 {
		return []*schema.ResourceData{d}, fmt.Errorf("Import format [gatewayID]/segmentID/bindingID expected, got %s", importID)
	}
	if len(s) == 3 {
		importGW = s[0]
		importSegment = s[1]
		d.SetId(s[2])
	} else {
		importSegment = s[0]
		d.SetId(s[1])
	}

	infra := "infra"
	if isPolicyGlobalManager(m) {
		infra = "global-infra"
	}

	parentPath := fmt.Sprintf("/%s/segments", infra)
	if len(importGW) > 0 {
		parentPath = fmt.Sprintf("/%s/tier-1s/%s/segments", infra, importGW)
	}
	segmentPath := fmt.Sprintf("%s/%s", parentPath, importSegment)
	d.Set("segment_path", segmentPath)

	return []*schema.ResourceData{d}, nil
}