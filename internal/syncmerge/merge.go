// Package syncmerge implements G6's deterministic, transport-independent
// metadata convergence rules. It consumes an unordered operation set and a
// snapshot baseline; callers persist and apply the resulting projection in the
// same transaction that advances their durable state vector.
package syncmerge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/renesugar/notrios/internal/syncstate"
)

const (
	RecoveredNotebookID = "nb_recovered"
	DefaultCollectionID = "default"
)

type Order struct {
	WallMS    int64
	Logical   int64
	ReplicaID string
	Sequence  int64
}

func OrderOf(operation syncstate.Operation) Order {
	return Order{operation.HLC.WallMS, operation.HLC.Logical, operation.ReplicaID, operation.Sequence}
}

func Compare(left, right Order) int {
	switch {
	case left.WallMS < right.WallMS:
		return -1
	case left.WallMS > right.WallMS:
		return 1
	case left.Logical < right.Logical:
		return -1
	case left.Logical > right.Logical:
		return 1
	case left.ReplicaID < right.ReplicaID:
		return -1
	case left.ReplicaID > right.ReplicaID:
		return 1
	case left.Sequence < right.Sequence:
		return -1
	case left.Sequence > right.Sequence:
		return 1
	default:
		return 0
	}
}

type Register struct {
	Value json.RawMessage
	Order Order
}

type Record struct {
	Type           string
	ID             string
	Fields         map[string]Register
	Active         bool
	LifecycleOrder Order
	Baseline       bool
}

type Membership struct {
	ID       string
	Document string
	Tag      string
	Present  bool
	Order    Order
}

type DeathCertificate struct {
	DocumentID string
	Signer     string
	Signature  string
	Order      Order
}

type Repair struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	SubjectID string          `json:"subject_id"`
	Details   json.RawMessage `json:"details"`
	Order     Order           `json:"order"`
}

type Projection struct {
	Records            map[string]*Record
	Memberships        map[string]Membership
	Deaths             map[string]DeathCertificate
	Repairs            []Repair
	NotebookName       map[string]string
	RecordName         map[string]string
	NotebookParent     map[string]string
	DocumentNotebook   map[string]string
	DocumentCollection map[string]string
}

func key(recordType, recordID string) string { return recordType + "\x00" + recordID }

var scalarFields = map[string]map[string]bool{
	"collection":      {"name": true, "description": true},
	"document":        {"collection_id": true, "notebook_id": true, "title": true, "body_mime_type": true},
	"notebook":        {"parent_id": true, "name": true, "icon_emoji": true, "position": true},
	"tag":             {"name": true},
	"search_notebook": {"name": true, "icon_emoji": true, "query": true},
	"document_source": {"source_system": true, "external_id": true, "author": true, "author_id": true, "thread_id": true, "reply_to": true, "source_url": true, "published_at": true, "metadata_json": true},
}

// Converge folds exactly the G6-owned records. Revision bodies, resource bytes,
// and their links remain in later slices and are ignored here.
func Converge(baseline []*Record, baselineMemberships []Membership, operations []syncstate.Operation) (Projection, error) {
	return ConvergeWithDeaths(baseline, baselineMemberships, nil, operations)
}

// ConvergeWithDeaths starts from checkpointed permanent-delete identities as
// well as ordinary baseline rows. G17 can therefore compact an acknowledged
// purge operation without ever making resurrection possible.
func ConvergeWithDeaths(baseline []*Record, baselineMemberships []Membership, baselineDeaths []DeathCertificate, operations []syncstate.Operation) (Projection, error) {
	p := Projection{
		Records: make(map[string]*Record), Memberships: make(map[string]Membership),
		Deaths: make(map[string]DeathCertificate), NotebookName: make(map[string]string), RecordName: make(map[string]string),
		NotebookParent: make(map[string]string), DocumentNotebook: make(map[string]string),
		DocumentCollection: make(map[string]string),
	}
	for _, death := range baselineDeaths {
		if death.DocumentID == "" || death.Signer == "" || death.Order.Sequence <= 0 || death.Signature == "" {
			return Projection{}, fmt.Errorf("invalid baseline death certificate")
		}
		p.Deaths[death.DocumentID] = death
	}
	for _, source := range baseline {
		if source == nil || scalarFields[source.Type] == nil || source.ID == "" {
			return Projection{}, fmt.Errorf("invalid metadata baseline record")
		}
		copyRecord := &Record{Type: source.Type, ID: source.ID, Active: source.Active, Baseline: true, Fields: make(map[string]Register)}
		for field, register := range source.Fields {
			if !scalarFields[source.Type][field] {
				return Projection{}, fmt.Errorf("invalid baseline field %s.%s", source.Type, field)
			}
			copyRecord.Fields[field] = Register{Value: append(json.RawMessage(nil), register.Value...), Order: register.Order}
		}
		p.Records[key(source.Type, source.ID)] = copyRecord
	}
	for _, membership := range baselineMemberships {
		if membership.ID == "" || membership.Document == "" || membership.Tag == "" {
			return Projection{}, fmt.Errorf("invalid metadata baseline membership")
		}
		membership.Order = Order{}
		p.Memberships[membership.ID] = membership
	}

	ordered := append([]syncstate.Operation(nil), operations...)
	sort.Slice(ordered, func(i, j int) bool { return Compare(OrderOf(ordered[i]), OrderOf(ordered[j])) < 0 })
	for _, operation := range ordered {
		if err := apply(&p, operation); err != nil {
			return Projection{}, fmt.Errorf("apply %s: %w", operation.OperationID, err)
		}
	}
	repairTreeAndNames(&p)
	repairFlatNames(&p, "tag", "tag.name_collision")
	repairFlatNames(&p, "search_notebook", "search_notebook.name_collision")
	repairDocumentHomes(&p)
	filterMemberships(&p)
	sort.Slice(p.Repairs, func(i, j int) bool { return p.Repairs[i].ID < p.Repairs[j].ID })
	return p, nil
}

func apply(p *Projection, operation syncstate.Operation) error {
	if operation.RecordType == "sync_noop" || scalarFields[operation.RecordType] == nil && operation.RecordType != "document_tag" {
		return nil
	}
	order := OrderOf(operation)
	if operation.RecordType == "document_tag" {
		return applyMembership(p, operation, order)
	}
	if protectedRecord(operation.RecordType, operation.RecordID) {
		return fmt.Errorf("protected record %s/%s cannot be synchronized as a mutation", operation.RecordType, operation.RecordID)
	}
	if death, found := p.Deaths[operation.RecordID]; found && operation.RecordType == "document" && operation.Kind != "document.purge" {
		return fmt.Errorf("document %q has death certificate %s:%d", operation.RecordID, death.Order.ReplicaID, death.Order.Sequence)
	}
	recordKey := key(operation.RecordType, operation.RecordID)
	record := p.Records[recordKey]
	if record == nil {
		record = &Record{Type: operation.RecordType, ID: operation.RecordID, Fields: make(map[string]Register)}
		p.Records[recordKey] = record
	}
	switch operation.Kind {
	case "record.create":
		if Compare(order, record.LifecycleOrder) > 0 {
			record.Active, record.LifecycleOrder = true, order
		}
	case "record.update":
	case "record.delete":
		if Compare(order, record.LifecycleOrder) > 0 {
			record.Active, record.LifecycleOrder = false, order
		}
	case "document.trash":
		if operation.RecordType != "document" {
			return fmt.Errorf("trash is only valid for documents")
		}
		if Compare(order, record.LifecycleOrder) > 0 {
			record.Active, record.LifecycleOrder = false, order
		}
	case "document.restore":
		if operation.RecordType != "document" {
			return fmt.Errorf("restore is only valid for documents")
		}
		if Compare(order, record.LifecycleOrder) > 0 {
			record.Active, record.LifecycleOrder = true, order
		}
	case "document.purge":
		if operation.RecordType != "document" {
			return fmt.Errorf("purge is only valid for documents")
		}
		certificate, err := parseCertificate(operation)
		if err != nil {
			return err
		}
		p.Deaths[operation.RecordID] = certificate
		record.Active, record.LifecycleOrder = false, order
		return nil
	default:
		return nil
	}
	if operation.Kind == "record.delete" {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(operation.Payload, &fields); err != nil {
		return fmt.Errorf("payload object: %w", err)
	}
	for field, value := range fields {
		if !scalarFields[operation.RecordType][field] {
			continue
		}
		if len(value) > 64<<10 || !json.Valid(value) {
			return fmt.Errorf("invalid field %q", field)
		}
		prior, exists := record.Fields[field]
		if !exists || Compare(order, prior.Order) > 0 {
			record.Fields[field] = Register{Value: append(json.RawMessage(nil), value...), Order: order}
		}
	}
	return nil
}

func applyMembership(p *Projection, operation syncstate.Operation, order Order) error {
	if operation.Kind != "membership.add" && operation.Kind != "membership.remove" {
		return nil
	}
	var payload struct {
		DocumentID string `json:"document_id"`
		TagID      string `json:"tag_id"`
	}
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return err
	}
	if payload.DocumentID == "" || payload.TagID == "" || operation.RecordID != payload.DocumentID+":"+payload.TagID {
		return fmt.Errorf("invalid document-tag membership identity")
	}
	prior, exists := p.Memberships[operation.RecordID]
	if !exists || Compare(order, prior.Order) > 0 {
		p.Memberships[operation.RecordID] = Membership{ID: operation.RecordID, Document: payload.DocumentID, Tag: payload.TagID, Present: operation.Kind == "membership.add", Order: order}
	}
	return nil
}

func parseCertificate(operation syncstate.Operation) (DeathCertificate, error) {
	var payload struct {
		DocumentID string `json:"document_id"`
		Signer     string `json:"signer_replica_id"`
		Sequence   int64  `json:"sequence"`
		Signature  string `json:"signature"`
	}
	if err := json.Unmarshal(operation.Payload, &payload); err != nil {
		return DeathCertificate{}, err
	}
	if payload.DocumentID != operation.RecordID || payload.Signer != operation.ReplicaID || payload.Sequence != operation.Sequence || payload.Signature == "" || len(payload.Signature) > 2048 {
		return DeathCertificate{}, fmt.Errorf("invalid structural death certificate")
	}
	return DeathCertificate{DocumentID: payload.DocumentID, Signer: payload.Signer, Signature: payload.Signature, Order: OrderOf(operation)}, nil
}

func repairTreeAndNames(p *Projection) {
	type edge struct {
		id, requested string
		order         Order
	}
	var edges []edge
	active := map[string]bool{}
	for _, record := range p.Records {
		if record.Type != "notebook" || !record.Active {
			continue
		}
		active[record.ID] = true
		parent, _ := stringField(record, "parent_id")
		reg := record.Fields["parent_id"]
		edges = append(edges, edge{record.ID, parent, reg.Order})
	}
	sort.Slice(edges, func(i, j int) bool {
		if compared := Compare(edges[i].order, edges[j].order); compared != 0 {
			return compared > 0
		}
		return edges[i].id < edges[j].id
	})
	parents := map[string]string{}
	for _, candidate := range edges {
		effective := candidate.requested
		kind := ""
		if effective != "" && !active[effective] {
			if active[RecoveredNotebookID] && candidate.id != RecoveredNotebookID {
				effective = RecoveredNotebookID
			} else {
				effective = ""
			}
			kind = "notebook.orphan"
		}
		if effective == candidate.id || createsCycle(candidate.id, effective, parents) {
			effective = nearestValidAncestor(candidate.id, effective, parents)
			kind = "notebook.cycle"
		}
		parents[candidate.id] = effective
		p.NotebookParent[candidate.id] = effective
		if kind != "" {
			addRepair(p, kind, candidate.id, candidate.requested, effective, candidate.order)
		}
	}

	type named struct {
		id, requested string
		order         Order
	}
	groups := map[string][]named{}
	for _, record := range p.Records {
		if record.Type != "notebook" || !record.Active {
			continue
		}
		name, _ := stringField(record, "name")
		groups[p.NotebookParent[record.ID]+"\x00"+strings.ToLower(name)] = append(groups[p.NotebookParent[record.ID]+"\x00"+strings.ToLower(name)], named{record.ID, name, record.Fields["name"].Order})
	}
	used := map[string]bool{}
	groupKeys := make([]string, 0, len(groups))
	for groupKey := range groups {
		groupKeys = append(groupKeys, groupKey)
	}
	sort.Strings(groupKeys)
	for _, groupKey := range groupKeys {
		group := groups[groupKey]
		sort.Slice(group, func(i, j int) bool {
			leftProtected := protectedRecord("notebook", group[i].id)
			rightProtected := protectedRecord("notebook", group[j].id)
			if leftProtected != rightProtected {
				return leftProtected
			}
			if c := Compare(group[i].order, group[j].order); c != 0 {
				return c > 0
			}
			return group[i].id < group[j].id
		})
		for index, item := range group {
			name := item.requested
			if index > 0 || used[p.NotebookParent[item.id]+"\x00"+strings.ToLower(name)] {
				name = stableName(item.requested, item.id, p.NotebookParent[item.id], used)
				addRepair(p, "notebook.name_collision", item.id, item.requested, name, item.order)
			}
			p.NotebookName[item.id] = name
			p.RecordName[key("notebook", item.id)] = name
			used[p.NotebookParent[item.id]+"\x00"+strings.ToLower(name)] = true
		}
	}
}

func repairFlatNames(p *Projection, recordType, repairKind string) {
	type named struct {
		id, requested string
		order         Order
	}
	groups := map[string][]named{}
	for _, record := range p.Records {
		if record.Type != recordType || !record.Active {
			continue
		}
		name, _ := stringField(record, "name")
		groups[strings.ToLower(name)] = append(groups[strings.ToLower(name)], named{record.ID, name, record.Fields["name"].Order})
	}
	groupKeys := make([]string, 0, len(groups))
	for groupKey := range groups {
		groupKeys = append(groupKeys, groupKey)
	}
	sort.Strings(groupKeys)
	used := map[string]bool{}
	for _, groupKey := range groupKeys {
		group := groups[groupKey]
		sort.Slice(group, func(i, j int) bool {
			leftProtected := protectedRecord(recordType, group[i].id)
			rightProtected := protectedRecord(recordType, group[j].id)
			if leftProtected != rightProtected {
				return leftProtected
			}
			if c := Compare(group[i].order, group[j].order); c != 0 {
				return c > 0
			}
			return group[i].id < group[j].id
		})
		for index, item := range group {
			name := item.requested
			if index > 0 || used["\x00"+strings.ToLower(name)] {
				name = stableName(item.requested, item.id, "", used)
				addRepair(p, repairKind, item.id, item.requested, name, item.order)
			}
			p.RecordName[key(recordType, item.id)] = name
			used["\x00"+strings.ToLower(name)] = true
		}
	}
}

func protectedRecord(recordType, recordID string) bool {
	switch recordType + "/" + recordID {
	case "collection/default", "notebook/nb_notes", "notebook/nb_help", "notebook/nb_reports", "notebook/nb_recovered", "search_notebook/snb_all_notes", "search_notebook/snb_trash":
		return true
	default:
		return false
	}
}

func createsCycle(child, parent string, parents map[string]string) bool {
	for parent != "" {
		if parent == child {
			return true
		}
		parent = parents[parent]
	}
	return false
}

func nearestValidAncestor(child, parent string, parents map[string]string) string {
	seen := map[string]bool{child: true}
	for parent != "" {
		if seen[parent] {
			return ""
		}
		seen[parent] = true
		parent = parents[parent]
	}
	return ""
}

func repairDocumentHomes(p *Projection) {
	activeNotebooks, activeCollections := map[string]bool{}, map[string]bool{}
	for _, record := range p.Records {
		if !record.Active {
			continue
		}
		if record.Type == "notebook" {
			activeNotebooks[record.ID] = true
		}
		if record.Type == "collection" {
			activeCollections[record.ID] = true
		}
	}
	for _, record := range p.Records {
		if record.Type != "document" {
			continue
		}
		notebook, _ := stringField(record, "notebook_id")
		if notebook != "" && !activeNotebooks[notebook] {
			effective := ""
			if activeNotebooks[RecoveredNotebookID] {
				effective = RecoveredNotebookID
			}
			p.DocumentNotebook[record.ID] = effective
			addRepair(p, "document.notebook_orphan", record.ID, notebook, effective, record.Fields["notebook_id"].Order)
		} else {
			p.DocumentNotebook[record.ID] = notebook
		}
		collection, _ := stringField(record, "collection_id")
		if !activeCollections[collection] && activeCollections[DefaultCollectionID] {
			p.DocumentCollection[record.ID] = DefaultCollectionID
			addRepair(p, "document.collection_orphan", record.ID, collection, DefaultCollectionID, record.Fields["collection_id"].Order)
		} else {
			p.DocumentCollection[record.ID] = collection
		}
	}
}

func filterMemberships(p *Projection) {
	for id, membership := range p.Memberships {
		if !membership.Present {
			continue
		}
		doc := p.Records[key("document", membership.Document)]
		tag := p.Records[key("tag", membership.Tag)]
		if doc == nil || tag == nil || !tag.Active {
			membership.Present = false
			p.Memberships[id] = membership
		}
	}
}

func addRepair(p *Projection, kind, subject, requested, effective string, order Order) {
	details, _ := json.Marshal(map[string]string{"effective": effective, "requested": requested})
	digest := sha256.Sum256([]byte(kind + "\x00" + subject + "\x00" + string(details) + "\x00" + fmt.Sprint(order)))
	p.Repairs = append(p.Repairs, Repair{ID: "repair_" + hex.EncodeToString(digest[:12]), Kind: kind, SubjectID: subject, Details: details, Order: order})
}

func stringField(record *Record, field string) (string, bool) {
	register, ok := record.Fields[field]
	if !ok {
		return "", false
	}
	var value string
	if json.Unmarshal(register.Value, &value) != nil || !utf8.ValidString(value) {
		return "", false
	}
	return value, true
}

func stableName(base, id, parent string, used map[string]bool) string {
	clean := id
	if len(clean) > 8 {
		clean = clean[:8]
	}
	name := base + " · " + clean
	for suffix := 2; used[parent+"\x00"+strings.ToLower(name)]; suffix++ {
		name = fmt.Sprintf("%s · %s-%d", base, clean, suffix)
	}
	return name
}

func String(record *Record, field, fallback string) string {
	if value, ok := stringField(record, field); ok {
		return value
	}
	return fallback
}

func Int(record *Record, field string, fallback int64) int64 {
	register, ok := record.Fields[field]
	if !ok {
		return fallback
	}
	var value int64
	if json.Unmarshal(register.Value, &value) != nil {
		return fallback
	}
	return value
}
