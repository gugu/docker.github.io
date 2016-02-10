package data

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/Sirupsen/logrus"
)

// Canonical base role names
const (
	CanonicalRootRole      = "root"
	CanonicalTargetsRole   = "targets"
	CanonicalSnapshotRole  = "snapshot"
	CanonicalTimestampRole = "timestamp"
)

// BaseRoles is an easy to iterate list of the top level
// roles.
var BaseRoles = []string{
	CanonicalRootRole,
	CanonicalTargetsRole,
	CanonicalSnapshotRole,
	CanonicalTimestampRole,
}

// Regex for validating delegation names
var delegationRegexp = regexp.MustCompile("^[-a-z0-9_/]+$")

// ErrNoSuchRole indicates the roles doesn't exist
type ErrNoSuchRole struct {
	Role string
}

func (e ErrNoSuchRole) Error() string {
	return fmt.Sprintf("role does not exist: %s", e.Role)
}

// ErrInvalidRole represents an error regarding a role. Typically
// something like a role for which sone of the public keys were
// not found in the TUF repo.
type ErrInvalidRole struct {
	Role   string
	Reason string
}

func (e ErrInvalidRole) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("tuf: invalid role %s. %s", e.Role, e.Reason)
	}
	return fmt.Sprintf("tuf: invalid role %s.", e.Role)
}

// ValidRole only determines the name is semantically
// correct. For target delegated roles, it does NOT check
// the the appropriate parent roles exist.
func ValidRole(name string) bool {
	if IsDelegation(name) {
		return true
	}

	for _, v := range BaseRoles {
		if name == v {
			return true
		}
	}
	return false
}

// IsBaseRole checks if a role name is valid base role
func IsBaseRole(name string) bool {
	for _, v := range BaseRoles {
		if name == v {
			return true
		}
	}
	return false
}

// IsDelegation checks if the role is a delegation or a root role
func IsDelegation(role string) bool {
	targetsBase := CanonicalTargetsRole + "/"

	whitelistedChars := delegationRegexp.MatchString(role)

	// Limit size of full role string to 255 chars for db column size limit
	correctLength := len(role) < 256

	// Removes ., .., extra slashes, and trailing slash
	isClean := path.Clean(role) == role
	return strings.HasPrefix(role, targetsBase) &&
		whitelistedChars &&
		correctLength &&
		isClean
}

// RootRole is a cut down role as it appears in the root.json
// Eventually should only be used for immediately before and after serialization/deserialization
type RootRole struct {
	KeyIDs    []string `json:"keyids"`
	Threshold int      `json:"threshold"`
}

// Role is a more verbose role as they appear in targets delegations
// Eventually should only be used for immediately before and after serialization/deserialization
type Role struct {
	RootRole
	Name             string   `json:"name"`
	Paths            []string `json:"paths,omitempty"`
	PathHashPrefixes []string `json:"path_hash_prefixes,omitempty"`
	Email            string   `json:"email,omitempty"`
}

// BaseRole is an internal representation of a root/targets/snapshot/timestamp role, with its public keys included
type BaseRole struct {
	Keys      map[string]PublicKey `json:"keys"`
	Name      string               `json:"name"`
	Threshold int                  `json:"threshold"`
}

// Returns true for BaseRole
func (b BaseRole) IsBaseRole() bool {
	return true
}

// Returns false for BaseRole
func (b BaseRole) IsDelegationRole() bool {
	return false
}

// GetName retrieves the name of this role
func (b BaseRole) GetName() string {
	return b.Name
}

// GetThreshold retrieves the threshold of this role
func (b BaseRole) GetThreshold() int {
	return b.Threshold
}

// ListKeys retrieves the public keys valid for this role
func (b BaseRole) ListKeys() KeyList {
	keys := make(KeyList)
	for _, key := range b.Keys {
		keys = append(keys, key)
	}
	return keys
}

// ListKeyIDs retrieves the list of key IDs valid for this role
func (b BaseRole) ListKeyIDs() []string {
	keyIDs := make([]string)
	for id := range b.Keys {
		keyIDs = append(keyIDs, id)
	}
	return keyIDs
}

// ListPaths returns an error for non-delegations
func (b BaseRole) ListPaths() ([]string, error) {
	return nil, fmt.Errorf("%s is not a delegation role", b.Name)
}

// ListPaths returns an error for non-delegations
func (b BaseRole) ListPathHashPrefixes() ([]string, error) {
	return nil, fmt.Errorf("%s is not a delegation role", b.Name)
}

// DelegationRole is an internal representation of a delegation role, with its public keys included
type DelegationRole struct {
	BaseRole
	Paths            []string `json:"paths,omitempty"`
	PathHashPrefixes []string `json:"path_hash_prefixes,omitempty"`
}

// Returns false for DelegationRole
func (d DelegationRole) IsBaseRole() bool {
	return false
}

// Returns true for DelegationRole
func (d DelegationRole) IsDelegationRole() bool {
	return true
}

// GetName retrieves the name of this role
func (d DelegationRole) GetName() string {
	return d.Name
}

// GetThreshold retrieves the threshold of this role
func (d DelegationRole) GetThreshold() int {
	return d.Threshold
}

// ListKeys retrieves the public keys valid for this role
func (d DelegationRole) ListKeys() KeyList {
	keys := make(KeyList)
	for _, key := range d.Keys {
		keys = append(keys, key)
	}
	return keys
}

// ListKeyIDs retrieves the list of key IDs valid for this role
func (d DelegationRole) ListKeyIDs() []string {
	keyIDs := make([]string)
	for id := range d.Keys {
		keyIDs = append(keyIDs, id)
	}
	return keyIDs
}

// ListPaths lists the paths of this role
func (d DelegationRole) ListPaths() ([]string, error) {
	return d.Paths, nil
}

// ListPaths lists the paths of this rol
func (d DelegationRole) ListPathHashPrefixes() ([]string, error) {
	return d.PathHashPrefixes, nil
}

// NewRole creates a new Role object from the given parameters
func NewRole(name string, threshold int, keyIDs, paths, pathHashPrefixes []string) (*Role, error) {
	if len(paths) > 0 && len(pathHashPrefixes) > 0 {
		return nil, ErrInvalidRole{
			Role:   name,
			Reason: "roles may not have both Paths and PathHashPrefixes",
		}
	}
	if IsDelegation(name) {
		if len(paths) == 0 && len(pathHashPrefixes) == 0 {
			logrus.Debugf("role %s with no Paths and no PathHashPrefixes will never be able to publish content until one or more are added", name)
		}
	}
	if threshold < 1 {
		return nil, ErrInvalidRole{Role: name}
	}
	if !ValidRole(name) {
		return nil, ErrInvalidRole{Role: name}
	}
	return &Role{
		RootRole: RootRole{
			KeyIDs:    keyIDs,
			Threshold: threshold,
		},
		Name:             name,
		Paths:            paths,
		PathHashPrefixes: pathHashPrefixes,
	}, nil

}

// IsValid checks if the role has defined both paths and path hash prefixes,
// having both is invalid
func (r Role) IsValid() bool {
	return !(len(r.Paths) > 0 && len(r.PathHashPrefixes) > 0)
}

// ValidKey checks if the given id is a recognized signing key for the role
func (r Role) ValidKey(id string) bool {
	for _, key := range r.KeyIDs {
		if key == id {
			return true
		}
	}
	return false
}

// CheckPaths checks if a given path is valid for the role
func (r Role) CheckPaths(path string) bool {
	for _, p := range r.Paths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// CheckPrefixes checks if a given hash matches the prefixes for the role
func (r Role) CheckPrefixes(hash string) bool {
	for _, p := range r.PathHashPrefixes {
		if strings.HasPrefix(hash, p) {
			return true
		}
	}
	return false
}

// IsDelegation checks if the role is a delegation or a root role
func (r Role) IsDelegation() bool {
	return IsDelegation(r.Name)
}

// AddKeys merges the ids into the current list of role key ids
func (r *Role) AddKeys(ids []string) {
	r.KeyIDs = mergeStrSlices(r.KeyIDs, ids)
}

// AddPaths merges the paths into the current list of role paths
func (r *Role) AddPaths(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	if len(r.PathHashPrefixes) > 0 {
		return ErrInvalidRole{Role: r.Name, Reason: "attempted to add paths to role that already has hash prefixes"}
	}
	r.Paths = mergeStrSlices(r.Paths, paths)
	return nil
}

// AddPathHashPrefixes merges the prefixes into the list of role path hash prefixes
func (r *Role) AddPathHashPrefixes(prefixes []string) error {
	if len(prefixes) == 0 {
		return nil
	}
	if len(r.Paths) > 0 {
		return ErrInvalidRole{Role: r.Name, Reason: "attempted to add hash prefixes to role that already has paths"}
	}
	r.PathHashPrefixes = mergeStrSlices(r.PathHashPrefixes, prefixes)
	return nil
}

// RemoveKeys removes the ids from the current list of key ids
func (r *Role) RemoveKeys(ids []string) {
	r.KeyIDs = subtractStrSlices(r.KeyIDs, ids)
}

// RemovePaths removes the paths from the current list of role paths
func (r *Role) RemovePaths(paths []string) {
	r.Paths = subtractStrSlices(r.Paths, paths)
}

// RemovePathHashPrefixes removes the prefixes from the current list of path hash prefixes
func (r *Role) RemovePathHashPrefixes(prefixes []string) {
	r.PathHashPrefixes = subtractStrSlices(r.PathHashPrefixes, prefixes)
}

func mergeStrSlices(orig, new []string) []string {
	have := make(map[string]bool)
	for _, e := range orig {
		have[e] = true
	}
	merged := make([]string, len(orig), len(orig)+len(new))
	copy(merged, orig)
	for _, e := range new {
		if !have[e] {
			merged = append(merged, e)
		}
	}
	return merged
}

func subtractStrSlices(orig, remove []string) []string {
	kill := make(map[string]bool)
	for _, e := range remove {
		kill[e] = true
	}
	var keep []string
	for _, e := range orig {
		if !kill[e] {
			keep = append(keep, e)
		}
	}
	return keep
}

// RoleWithKeys is a role that has the signing keys for the role embedded
type RoleWithKeys struct {
	Role
	Keys Keys
}
