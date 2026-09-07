package gtm

import (
	"context"
	"fmt"

	tagmanager "google.golang.org/api/tagmanager/v2"
)

// isClickLinkFormTrigger returns true for trigger types where the GTM API
// silently ignores autoEventFilter and requires conditions in the filter field instead.
func isClickLinkFormTrigger(triggerType string) bool {
	return triggerType == "linkClick" || triggerType == "formSubmission" || triggerType == "click"
}

// CreateTag creates a new tag in the workspace.
func (c *Client) CreateTag(ctx context.Context, accountID, containerID, workspaceID string, input *TagInput) (*CreatedTag, error) {
	parent := BuildWorkspacePath(accountID, containerID, workspaceID)

	tag := &tagmanager.Tag{
		Name:              input.Name,
		Type:              input.Type,
		FiringTriggerId:   input.FiringTriggerId,
		BlockingTriggerId: input.BlockingTriggerId,
		Parameter:         toAPIParams(input.Parameter),
		Notes:             input.Notes,
		Paused:            input.Paused,
		TagFiringOption:   input.TagFiringOption,
		SetupTag:          toAPISetupTags(input.SetupTag),
		TeardownTag:       toAPITeardownTags(input.TeardownTag),
		ConsentSettings:   toAPIConsentSettings(input.ConsentStatus, input.ConsentTypes),
	}

	result, err := c.Service.Accounts.Containers.Workspaces.Tags.Create(parent, tag).Context(ctx).Do()
	if err != nil {
		return nil, mapGoogleError(err)
	}

	return &CreatedTag{
		TagID:       result.TagId,
		Name:        result.Name,
		Type:        result.Type,
		Path:        result.Path,
		Fingerprint: result.Fingerprint,
	}, nil
}

// UpdateTag updates an existing tag. It fetches the current tag first and merges
// only the fields that were explicitly provided, preserving everything else.
// The GTM API uses PUT (full replacement), so we must send the complete tag.
func (c *Client) UpdateTag(ctx context.Context, path string, input *TagInput) (*CreatedTag, error) {
	// Get current tag — used as the base for the merged update
	current, err := c.Service.Accounts.Containers.Workspaces.Tags.Get(path).Context(ctx).Do()
	if err != nil {
		return nil, mapGoogleError(err)
	}

	// Start from the current tag and selectively override provided fields
	tag := current

	if input.Name != "" {
		tag.Name = input.Name
	}
	if input.Type != "" {
		tag.Type = input.Type
	}
	if input.FiringTriggerId != nil {
		tag.FiringTriggerId = input.FiringTriggerId
	}
	if input.BlockingTriggerId != nil {
		tag.BlockingTriggerId = input.BlockingTriggerId
	}
	if input.HasParameter {
		tag.Parameter = toAPIParams(input.Parameter)
	}
	if input.Notes != "" {
		tag.Notes = input.Notes
	}
	if input.HasPaused {
		tag.Paused = input.Paused
	}
	if input.TagFiringOption != "" {
		tag.TagFiringOption = input.TagFiringOption
	}

	// Setup/teardown tag handling
	if input.HasSetupTag {
		if input.ClearSetupTag {
			tag.SetupTag = []*tagmanager.SetupTag{}
		} else {
			tag.SetupTag = toAPISetupTags(input.SetupTag)
		}
	}
	if input.HasTeardownTag {
		if input.ClearTeardownTag {
			tag.TeardownTag = []*tagmanager.TeardownTag{}
		} else {
			tag.TeardownTag = toAPITeardownTags(input.TeardownTag)
		}
	}

	// Consent settings
	if input.HasConsentSettings {
		tag.ConsentSettings = toAPIConsentSettings(input.ConsentStatus, input.ConsentTypes)
	}

	result, err := c.Service.Accounts.Containers.Workspaces.Tags.Update(path, tag).Fingerprint(current.Fingerprint).Context(ctx).Do()
	if err != nil {
		return nil, mapGoogleError(err)
	}

	return &CreatedTag{
		TagID:       result.TagId,
		Name:        result.Name,
		Type:        result.Type,
		Path:        result.Path,
		Fingerprint: result.Fingerprint,
	}, nil
}

// DeleteTag deletes a tag from the workspace.
func (c *Client) DeleteTag(ctx context.Context, path string) error {
	err := c.Service.Accounts.Containers.Workspaces.Tags.Delete(path).Context(ctx).Do()
	return mapGoogleError(err)
}

// CreateTrigger creates a new trigger in the workspace.
func (c *Client) CreateTrigger(ctx context.Context, accountID, containerID, workspaceID string, input *TriggerInput) (*CreatedTrigger, error) {
	parent := BuildWorkspacePath(accountID, containerID, workspaceID)

	// GTM API silently ignores autoEventFilter for linkClick/formSubmission/click triggers.
	// These trigger types require conditions to be set via the filter field instead.
	filter := input.Filter
	autoEventFilter := input.AutoEventFilter
	if isClickLinkFormTrigger(input.Type) && len(autoEventFilter) > 0 && len(filter) == 0 {
		filter = autoEventFilter
		autoEventFilter = nil
	}

	trigger := &tagmanager.Trigger{
		Name:              input.Name,
		Type:              input.Type,
		Filter:            toAPIConditions(filter),
		AutoEventFilter:   toAPIConditions(autoEventFilter),
		CustomEventFilter: toAPIConditions(input.CustomEventFilter),
		Parameter:         toAPIParams(input.Parameter),
		Notes:             input.Notes,
	}

	if input.EventName != nil {
		trigger.EventName = toAPIParam(input.EventName)
	}

	// For click/form/link triggers with conditions, set required companion fields
	if len(input.AutoEventFilter) > 0 && isClickLinkFormTrigger(input.Type) {
		trigger.WaitForTags = &tagmanager.Parameter{Type: "boolean", Value: "false"}
		trigger.WaitForTagsTimeout = &tagmanager.Parameter{Type: "integer", Value: "2000"}
		trigger.CheckValidation = &tagmanager.Parameter{Type: "boolean", Value: "false"}
	}

	result, err := c.Service.Accounts.Containers.Workspaces.Triggers.Create(parent, trigger).Context(ctx).Do()
	if err != nil {
		return nil, mapGoogleError(err)
	}

	return &CreatedTrigger{
		TriggerID:   result.TriggerId,
		Name:        result.Name,
		Type:        result.Type,
		Path:        result.Path,
		Fingerprint: result.Fingerprint,
	}, nil
}

// DeleteTrigger deletes a trigger from the workspace.
func (c *Client) DeleteTrigger(ctx context.Context, path string) error {
	err := c.Service.Accounts.Containers.Workspaces.Triggers.Delete(path).Context(ctx).Do()
	return mapGoogleError(err)
}

// UpdateTrigger updates an existing trigger. It fetches the current trigger first to get the fingerprint.
// Fields not provided in input are preserved from the current trigger.
func (c *Client) UpdateTrigger(ctx context.Context, path string, input *TriggerInput) (*CreatedTrigger, error) {
	// Get current trigger for fingerprint and to preserve unset fields
	current, err := c.Service.Accounts.Containers.Workspaces.Triggers.Get(path).Context(ctx).Do()
	if err != nil {
		return nil, mapGoogleError(err)
	}

	// GTM API silently ignores autoEventFilter for linkClick/formSubmission/click triggers.
	// Remap to filter field for these trigger types when autoEventFilter is provided and filter is not.
	filterInput := input.Filter
	autoEventFilterInput := input.AutoEventFilter
	if isClickLinkFormTrigger(input.Type) && len(autoEventFilterInput) > 0 && len(filterInput) == 0 {
		filterInput = autoEventFilterInput
		autoEventFilterInput = nil
	}

	// Preserve omitted fields; a non-nil empty slice explicitly clears a field.
	var forceSendFields []string
	filter := toAPIConditions(filterInput)
	if filterInput == nil {
		filter = current.Filter
	} else if len(filterInput) == 0 {
		filter = []*tagmanager.Condition{}
		forceSendFields = append(forceSendFields, "Filter")
	}
	autoEventFilter := toAPIConditions(autoEventFilterInput)
	if autoEventFilterInput == nil {
		autoEventFilter = current.AutoEventFilter
	} else if len(autoEventFilterInput) == 0 {
		autoEventFilter = []*tagmanager.Condition{}
		forceSendFields = append(forceSendFields, "AutoEventFilter")
	}
	customEventFilter := toAPIConditions(input.CustomEventFilter)
	if input.CustomEventFilter == nil {
		customEventFilter = current.CustomEventFilter
	} else if len(input.CustomEventFilter) == 0 {
		customEventFilter = []*tagmanager.Condition{}
		forceSendFields = append(forceSendFields, "CustomEventFilter")
	}
	params := toAPIParams(input.Parameter)
	if input.Parameter == nil {
		params = current.Parameter
	} else if len(input.Parameter) == 0 {
		params = []*tagmanager.Parameter{}
		forceSendFields = append(forceSendFields, "Parameter")
	}

	trigger := &tagmanager.Trigger{
		ForceSendFields:   forceSendFields,
		Name:              input.Name,
		Type:              input.Type,
		Filter:            filter,
		AutoEventFilter:   autoEventFilter,
		CustomEventFilter: customEventFilter,
		Parameter:         params,
		Notes:             input.Notes,
		// Preserve trigger-specific fields from current trigger (exclude auto-generated ones)
		CheckValidation:                current.CheckValidation,
		WaitForTags:                    current.WaitForTags,
		WaitForTagsTimeout:             current.WaitForTagsTimeout,
		ContinuousTimeMinMilliseconds:  current.ContinuousTimeMinMilliseconds,
		HorizontalScrollPercentageList: current.HorizontalScrollPercentageList,
		Interval:                       current.Interval,
		IntervalSeconds:                current.IntervalSeconds,
		Limit:                          current.Limit,
		MaxTimerLengthSeconds:          current.MaxTimerLengthSeconds,
		Selector:                       current.Selector,
		TotalTimeMinMilliseconds:       current.TotalTimeMinMilliseconds,
		VerticalScrollPercentageList:   current.VerticalScrollPercentageList,
		VisibilitySelector:             current.VisibilitySelector,
		VisiblePercentageMax:           current.VisiblePercentageMax,
		VisiblePercentageMin:           current.VisiblePercentageMin,
	}
	// NOTE: Do NOT include UniqueTriggerId - it's auto-generated during output generation
	// NOTE: Fingerprint is passed as URL parameter, not in body

	if input.EventName != nil {
		trigger.EventName = toAPIParam(input.EventName)
	} else {
		trigger.EventName = current.EventName
	}

	// For click/form/link triggers with conditions (filter remapped from autoEventFilter),
	// ensure companion fields have correct boolean/integer types.
	if len(input.AutoEventFilter) > 0 && isClickLinkFormTrigger(input.Type) {
		trigger.WaitForTags = &tagmanager.Parameter{Type: "boolean", Value: "false"}
		trigger.WaitForTagsTimeout = &tagmanager.Parameter{Type: "integer", Value: "2000"}
		trigger.CheckValidation = &tagmanager.Parameter{Type: "boolean", Value: "false"}
	}

	result, err := c.Service.Accounts.Containers.Workspaces.Triggers.Update(path, trigger).Fingerprint(current.Fingerprint).Context(ctx).Do()
	if err != nil {
		return nil, mapGoogleError(err)
	}

	return &CreatedTrigger{
		TriggerID:   result.TriggerId,
		Name:        result.Name,
		Type:        result.Type,
		Path:        result.Path,
		Fingerprint: result.Fingerprint,
	}, nil
}

// CreateVariable creates a new variable in the workspace.
func (c *Client) CreateVariable(ctx context.Context, accountID, containerID, workspaceID string, input *VariableInput) (*CreatedVariable, error) {
	parent := BuildWorkspacePath(accountID, containerID, workspaceID)

	variable := &tagmanager.Variable{
		Name:      input.Name,
		Type:      input.Type,
		Parameter: toAPIParams(input.Parameter),
		Notes:     input.Notes,
	}

	result, err := c.Service.Accounts.Containers.Workspaces.Variables.Create(parent, variable).Context(ctx).Do()
	if err != nil {
		return nil, mapGoogleError(err)
	}

	return &CreatedVariable{
		VariableID:  result.VariableId,
		Name:        result.Name,
		Type:        result.Type,
		Path:        result.Path,
		Fingerprint: result.Fingerprint,
	}, nil
}

// UpdateVariable updates an existing variable. It fetches the current variable first to get the fingerprint.
func (c *Client) UpdateVariable(ctx context.Context, path string, input *VariableInput) (*CreatedVariable, error) {
	// Get current variable for fingerprint
	current, err := c.Service.Accounts.Containers.Workspaces.Variables.Get(path).Context(ctx).Do()
	if err != nil {
		return nil, mapGoogleError(err)
	}

	variable := &tagmanager.Variable{
		Name:      input.Name,
		Type:      input.Type,
		Parameter: toAPIParams(input.Parameter),
		Notes:     input.Notes,
	}

	result, err := c.Service.Accounts.Containers.Workspaces.Variables.Update(path, variable).Fingerprint(current.Fingerprint).Context(ctx).Do()
	if err != nil {
		return nil, mapGoogleError(err)
	}

	return &CreatedVariable{
		VariableID:  result.VariableId,
		Name:        result.Name,
		Type:        result.Type,
		Path:        result.Path,
		Fingerprint: result.Fingerprint,
	}, nil
}

// DeleteVariable deletes a variable from the workspace.
func (c *Client) DeleteVariable(ctx context.Context, path string) error {
	err := c.Service.Accounts.Containers.Workspaces.Variables.Delete(path).Context(ctx).Do()
	return mapGoogleError(err)
}

func toAPIParams(params []Parameter) []*tagmanager.Parameter {
	if len(params) == 0 {
		return nil
	}
	result := make([]*tagmanager.Parameter, len(params))
	for i, p := range params {
		result[i] = toAPIParam(&p)
	}
	return result
}

func toAPIParam(p *Parameter) *tagmanager.Parameter {
	if p == nil {
		return nil
	}
	param := &tagmanager.Parameter{
		Type:            p.Type,
		Key:             p.Key,
		Value:           p.Value,
		ForceSendFields: []string{"Type", "Key", "Value"},
	}
	if len(p.List) > 0 {
		param.List = toAPIParams(p.List)
	}
	if len(p.Map) > 0 {
		param.Map = toAPIParams(p.Map)
	}
	return param
}

func toAPIConsentSettings(status string, types []string) *tagmanager.TagConsentSetting {
	if status == "" {
		return nil
	}
	cs := &tagmanager.TagConsentSetting{
		ConsentStatus:   status,
		ForceSendFields: []string{"ConsentStatus"},
	}
	if status == "needed" && len(types) > 0 {
		list := make([]*tagmanager.Parameter, len(types))
		for i, t := range types {
			list[i] = &tagmanager.Parameter{
				Type:  "template",
				Value: t,
			}
		}
		cs.ConsentType = &tagmanager.Parameter{
			Type: "list",
			List: list,
		}
	}
	return cs
}

func toAPISetupTags(tags []SetupTagInput) []*tagmanager.SetupTag {
	if len(tags) == 0 {
		return nil
	}
	result := make([]*tagmanager.SetupTag, len(tags))
	for i, t := range tags {
		result[i] = &tagmanager.SetupTag{
			TagName:            t.TagName,
			StopOnSetupFailure: t.StopOnSetupFailure,
		}
	}
	return result
}

func toAPITeardownTags(tags []TeardownTagInput) []*tagmanager.TeardownTag {
	if len(tags) == 0 {
		return nil
	}
	result := make([]*tagmanager.TeardownTag, len(tags))
	for i, t := range tags {
		result[i] = &tagmanager.TeardownTag{
			TagName:               t.TagName,
			StopTeardownOnFailure: t.StopTeardownOnFailure,
		}
	}
	return result
}

func toAPIConditions(conditions []Condition) []*tagmanager.Condition {
	if len(conditions) == 0 {
		return nil
	}
	result := make([]*tagmanager.Condition, len(conditions))
	for i, c := range conditions {
		// Transform doesNotContain → contains + negate.
		// The GTM API does not accept doesNotContain directly; the GTM UI
		// represents "does not contain" as a contains condition with a negate parameter.
		if c.Type == "doesNotContain" {
			c.Type = "contains"
			c.Negate = true
		}

		params := toAPIParams(c.Parameter)
		if c.Negate {
			params = append(params, &tagmanager.Parameter{
				Type:  "boolean",
				Key:   "negate",
				Value: "true",
			})
		}
		result[i] = &tagmanager.Condition{
			Type:            c.Type,
			Parameter:       params,
			ForceSendFields: []string{"Type", "Parameter"},
		}
	}
	return result
}

// BuildTagPath constructs a tag path from IDs.
func BuildTagPath(accountID, containerID, workspaceID, tagID string) string {
	return fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/tags/%s",
		accountID, containerID, workspaceID, tagID)
}

// BuildTriggerPath constructs a trigger path from IDs.
func BuildTriggerPath(accountID, containerID, workspaceID, triggerID string) string {
	return fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/triggers/%s",
		accountID, containerID, workspaceID, triggerID)
}

// BuildVariablePath constructs a variable path from IDs.
func BuildVariablePath(accountID, containerID, workspaceID, variableID string) string {
	return fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/variables/%s",
		accountID, containerID, workspaceID, variableID)
}

// BuildClientPath constructs a client path from IDs.
func BuildClientPath(accountID, containerID, workspaceID, clientID string) string {
	return fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/clients/%s",
		accountID, containerID, workspaceID, clientID)
}

// BuildTransformationPath constructs a transformation path from IDs.
func BuildTransformationPath(accountID, containerID, workspaceID, transformationID string) string {
	return fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/transformations/%s",
		accountID, containerID, workspaceID, transformationID)
}
