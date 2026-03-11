package workspace

import "context"

// VMProvider provisions and destroys cloud VMs.
type VMProvider interface {
	Create(ctx context.Context, opts VMCreateOpts) (*VM, error)
	Delete(ctx context.Context, id string) error
}

// VM represents a provisioned cloud VM.
type VM struct {
	ID       string
	PublicIP string
	Status   string
}

// VMCreateOpts configures VM creation.
type VMCreateOpts struct {
	Name       string
	UserData   string
	ImageName  string // default: "ubuntu-24.04"
	ServerType string // default: "cx22"
}
