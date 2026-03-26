package gtm

import (
	"context"
	"fmt"

	tagmanager "google.golang.org/api/tagmanager/v2"
)

// TagSequenceRef represents a setup or teardown tag reference.
type TagSequenceRef struct {
	TagName            string `json:"tagName"`
	StopOnFailure      bool   `json:"stopOnFailure,omitempty"`
}

// TagConsentSettings represents consent configuration for a tag.
type TagConsentSettings struct {
	ConsentStatus string   `json:"consentStatus"`           // "notSet", "notNeeded", "needed"
	ConsentTypes  []string `json:"consentTypes,omitempty"`  // e.g. ["ad_storage", "analytics_storage"]
}

// Tag is a simplified representation of a GTM tag.
type Tag struct {
	TagID             string              `json:"tagId"`
	Name              string              `json:"name"`
	Type              string              `json:"type"`
	Parameter         any                 `json:"parameter,omitempty"`
	FiringTriggerID   []string            `json:"firingTriggerId,omitempty"`
	BlockingTriggerID []string            `json:"blockingTriggerId,omitempty"`
	SetupTag          []TagSequenceRef    `json:"setupTag,omitempty"`
	TeardownTag       []TagSequenceRef    `json:"teardownTag,omitempty"`
	ConsentSettings   *TagConsentSettings `json:"consentSettings,omitempty"`
	Paused            bool                `json:"paused,omitempty"`
	Path              string              `json:"path"`
}

// ListTags returns all tags in a workspace.
func (c *Client) ListTags(ctx context.Context, accountID, containerID, workspaceID string) ([]Tag, error) {
	parent := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s", accountID, containerID, workspaceID)

	resp, err := retryWithBackoff(ctx, 3, func() (*tagmanager.ListTagsResponse, error) {
		return c.Service.Accounts.Containers.Workspaces.Tags.List(parent).Context(ctx).Do()
	})
	if err != nil {
		return nil, mapGoogleError(err)
	}

	return toTags(resp.Tag), nil
}

// GetTag returns a specific tag by ID.
func (c *Client) GetTag(ctx context.Context, accountID, containerID, workspaceID, tagID string) (*Tag, error) {
	path := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/tags/%s",
		accountID, containerID, workspaceID, tagID)

	tag, err := retryWithBackoff(ctx, 3, func() (*tagmanager.Tag, error) {
		return c.Service.Accounts.Containers.Workspaces.Tags.Get(path).Context(ctx).Do()
	})
	if err != nil {
		return nil, mapGoogleError(err)
	}

	result := toTag(tag)
	return &result, nil
}

func toTags(tags []*tagmanager.Tag) []Tag {
	result := make([]Tag, 0, len(tags))
	for _, t := range tags {
		result = append(result, toTag(t))
	}
	return result
}

func toTag(t *tagmanager.Tag) Tag {
	tag := Tag{
		TagID:             t.TagId,
		Name:              t.Name,
		Type:              t.Type,
		FiringTriggerID:   t.FiringTriggerId,
		BlockingTriggerID: t.BlockingTriggerId,
		Paused:            t.Paused,
		Path:              t.Path,
	}
	if len(t.Parameter) > 0 {
		tag.Parameter = t.Parameter
	}
	for _, s := range t.SetupTag {
		tag.SetupTag = append(tag.SetupTag, TagSequenceRef{
			TagName:       s.TagName,
			StopOnFailure: s.StopOnSetupFailure,
		})
	}
	for _, s := range t.TeardownTag {
		tag.TeardownTag = append(tag.TeardownTag, TagSequenceRef{
			TagName:       s.TagName,
			StopOnFailure: s.StopTeardownOnFailure,
		})
	}
	if t.ConsentSettings != nil && t.ConsentSettings.ConsentStatus != "" {
		cs := &TagConsentSettings{
			ConsentStatus: t.ConsentSettings.ConsentStatus,
		}
		if t.ConsentSettings.ConsentType != nil {
			for _, p := range t.ConsentSettings.ConsentType.List {
				if p.Value != "" {
					cs.ConsentTypes = append(cs.ConsentTypes, p.Value)
				}
			}
		}
		tag.ConsentSettings = cs
	}
	return tag
}
