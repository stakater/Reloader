package reload

import (
	"encoding/json"
	"slices"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stakater/Reloader/pkg/matcher"
)

// PendingReload records on a workload a reload that its sync windows deferred, so the
// deferral survives a Reloader restart or a change of leader.
type PendingReload struct {
	// Kind is the type of the resource whose change is waiting, e.g. "configmap".
	Kind matcher.ResourceType `json:"kind"`
	// Name is the name of the resource, in the workload's namespace.
	Name string `json:"name"`
	// Hash is the resource content hash the reload was deferred for.
	Hash string `json:"hash"`
	// NotBefore is when the reload is due: the window opening plus the workload's spread.
	NotBefore time.Time `json:"notBefore"`
}

// matches reports whether the entry is for the given resource.
func (p PendingReload) matches(kind matcher.ResourceType, name string) bool {
	return p.Kind == kind && p.Name == name
}

// ParsePendingReloads parses the pending-reload annotation. An empty or unparseable value
// yields no entries; Reloader owns the annotation and rewrites it on the next deferral.
func ParsePendingReloads(value string) []PendingReload {
	if value == "" {
		return nil
	}
	var entries []PendingReload
	if err := json.Unmarshal([]byte(value), &entries); err != nil {
		return nil
	}
	return entries
}

// FindPendingReload returns the entry for the given resource in the workload annotations.
func FindPendingReload(annotations map[string]string, key string, kind matcher.ResourceType, name string) (PendingReload, bool) {
	for _, entry := range ParsePendingReloads(annotations[key]) {
		if entry.matches(kind, name) {
			return entry, true
		}
	}
	return PendingReload{}, false
}

// SetPendingReload records entry on obj, replacing any entry for the same resource, and
// reports whether the annotation changed.
func SetPendingReload(obj client.Object, key string, entry PendingReload) bool {
	annotations := obj.GetAnnotations()
	entries := ParsePendingReloads(annotations[key])

	entry.NotBefore = entry.NotBefore.UTC().Truncate(time.Second)
	i := slices.IndexFunc(entries, func(e PendingReload) bool { return e.matches(entry.Kind, entry.Name) })
	if i >= 0 {
		if e := entries[i]; e.Hash == entry.Hash && e.NotBefore.Equal(entry.NotBefore) {
			return false
		}
		entries[i] = entry
	} else {
		entries = append(entries, entry)
	}

	return writePendingReloads(obj, key, entries)
}

// ClearPendingReload removes the entry for the given resource from obj and reports whether
// the annotation changed. The annotation is removed once no entry is left.
func ClearPendingReload(obj client.Object, key string, kind matcher.ResourceType, name string) bool {
	annotations := obj.GetAnnotations()
	value, ok := annotations[key]
	if !ok {
		return false
	}
	entries := ParsePendingReloads(value)
	kept := slices.DeleteFunc(slices.Clone(entries), func(e PendingReload) bool { return e.matches(kind, name) })
	if len(kept) == len(entries) && entries != nil {
		return false
	}
	return writePendingReloads(obj, key, kept)
}

func writePendingReloads(obj client.Object, key string, entries []PendingReload) bool {
	annotations := obj.GetAnnotations()
	if len(entries) == 0 {
		if _, ok := annotations[key]; !ok {
			return false
		}
		delete(annotations, key)
		obj.SetAnnotations(annotations)
		return true
	}

	data, err := json.Marshal(entries)
	if err != nil {
		// A slice of plain structs always marshals.
		return false
	}
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if annotations[key] == string(data) {
		return false
	}
	annotations[key] = string(data)
	obj.SetAnnotations(annotations)
	return true
}
