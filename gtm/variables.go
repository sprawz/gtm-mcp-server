package gtm

import (
	"context"
	"fmt"

	tagmanager "google.golang.org/api/tagmanager/v2"
)

// Variable is a simplified representation of a GTM variable.
type Variable struct {
	VariableID string `json:"variableId"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Parameter  any    `json:"parameter,omitempty"`
	Path       string `json:"path"`
}

// ListVariables returns all variables in a workspace.
func (c *Client) ListVariables(ctx context.Context, accountID, containerID, workspaceID string) ([]Variable, error) {
	parent := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s", accountID, containerID, workspaceID)

	resp, err := retryWithBackoff(ctx, 3, func() (*tagmanager.ListVariablesResponse, error) {
		return c.Service.Accounts.Containers.Workspaces.Variables.List(parent).Context(ctx).Do()
	})
	if err != nil {
		return nil, mapGoogleError(err)
	}

	return toVariables(resp.Variable), nil
}

// GetVariable returns a specific variable by ID.
func (c *Client) GetVariable(ctx context.Context, accountID, containerID, workspaceID, variableID string) (*Variable, error) {
	path := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/variables/%s",
		accountID, containerID, workspaceID, variableID)

	v, err := retryWithBackoff(ctx, 3, func() (*tagmanager.Variable, error) {
		return c.Service.Accounts.Containers.Workspaces.Variables.Get(path).Context(ctx).Do()
	})
	if err != nil {
		return nil, mapGoogleError(err)
	}

	result := Variable{
		VariableID: v.VariableId,
		Name:       v.Name,
		Type:       v.Type,
		Path:       v.Path,
	}
	if len(v.Parameter) > 0 {
		result.Parameter = v.Parameter
	}
	return &result, nil
}

func toVariables(variables []*tagmanager.Variable) []Variable {
	result := make([]Variable, 0, len(variables))
	for _, v := range variables {
		variable := Variable{
			VariableID: v.VariableId,
			Name:       v.Name,
			Type:       v.Type,
			Path:       v.Path,
		}
		if len(v.Parameter) > 0 {
			variable.Parameter = v.Parameter
		}
		result = append(result, variable)
	}
	return result
}
