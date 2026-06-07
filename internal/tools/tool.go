package tools

import "context"

// Tool defines the interface for all agent tools.
type Tool interface {
	// Name returns the tool's identifier.
	Name() string
	// Description returns a human-readable description of what the tool does.
	Description() string
	// Execute runs the tool with the given input and returns the output.
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

// Registry holds all available tools.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tool names and descriptions.
func (r *Registry) List() []map[string]string {
	var result []map[string]string
	for _, t := range r.tools {
		result = append(result, map[string]string{
			"name":        t.Name(),
			"description": t.Description(),
		})
	}
	return result
}
