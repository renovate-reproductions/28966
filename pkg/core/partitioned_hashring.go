package core

import (
	"encoding/json"
	"log"

	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/persistence"
	pjson "gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/persistence/json"
)

type partitionedHashring struct {
	// partitions is a map of partition names (for example distributor names)
	// and their hashrings
	partitions map[string]*Hashring

	// relations are the resource identifiers (like fingerprints or IPs)
	// and their related partition names. Are used to place a new resource
	// in the partition where they are related, for example two bridges with
	// the same fingerprint should be in the same partition always.
	relations map[string]string

	stencil *stencil

	store          persistence.Mechanism
	storeResources bool
}

func newPartitionedHashring(proportions map[string]int) *partitionedHashring {
	stencil := buildStencil(proportions)
	p := partitionedHashring{
		partitions: make(map[string]*Hashring),
		relations:  make(map[string]string),
		stencil:    stencil,
	}
	for partitionName := range proportions {
		p.partitions[partitionName] = NewHashring()
	}
	return &p
}

func (p partitionedHashring) Add(resource Resource) error {
	name := p.getPartitionName(resource)
	p.addRelationIdentifiers(resource, name)
	hashring := p.partitions[name]
	return hashring.Add(resource)
}

func (p partitionedHashring) AddOrUpdate(resource Resource) int {
	name := p.getPartitionName(resource)
	p.addRelationIdentifiers(resource, name)
	hashring := p.partitions[name]
	return hashring.AddOrUpdate(resource)
}

func (p partitionedHashring) Remove(resource Resource) error {
	hashring := p.partitions[p.getPartitionName(resource)]
	return hashring.Remove(resource)
}

func (p partitionedHashring) Filter(f FilterFunc) []Resource {
	resources := []Resource{}
	for _, h := range p.partitions {
		resources = append(resources, h.Filter(f)...)
	}
	return resources
}

func (p partitionedHashring) GetAll() []Resource {
	resources := []Resource{}
	for _, h := range p.partitions {
		resources = append(resources, h.GetAll()...)
	}
	return resources
}

func (p partitionedHashring) Prune() []Resource {
	resources := []Resource{}
	for _, h := range p.partitions {
		resources = append(resources, h.Prune()...)
	}
	return resources
}

func (p partitionedHashring) Len() int {
	count := 0
	for _, partition := range p.partitions {
		count += partition.Len()
	}
	return count
}

func (p partitionedHashring) Clear() {
	for name := range p.partitions {
		p.partitions[name] = NewHashring()
	}
}

func (p partitionedHashring) getPartitionName(resource Resource) (partitionName string) {
	identifiers := resource.RelationIdentifiers()
	for _, id := range identifiers {
		name, ok := p.relations[id]
		if _, existPartition := p.partitions[name]; ok && existPartition {
			partitionName = name
		}
	}

	if partitionName == "" {
		partitionName = p.stencil.GetPartitionName(resource)
	}
	return
}

func (p partitionedHashring) getHashring(partitionName string) *Hashring {
	return p.partitions[partitionName]
}

func (p partitionedHashring) addRelationIdentifiers(resource Resource, partitionName string) {
	for _, identifier := range resource.RelationIdentifiers() {
		p.relations[identifier] = partitionName
	}
}

type storeData struct {
	Relations map[string]string
	Resources []Resource
}

func (p *partitionedHashring) initStore(name string, dir string, storeResources bool, newResource func() Resource) {
	p.store = pjson.New(name, dir)
	p.storeResources = storeResources

	var data struct {
		Relations map[string]string
		Resources []json.RawMessage
	}

	err := p.store.Load(&data)
	if err != nil {
		log.Println("Error loading data from", name, "hashring store:", err)
		return
	}
	p.relations = data.Relations
	if storeResources {
		for _, rawResource := range data.Resources {
			resource := newResource()
			err := json.Unmarshal(rawResource, &resource)
			if err != nil {
				log.Println("Error loading resource from", name, "hashring store:", err)
				continue
			}
			p.Add(resource)
		}
	}
}

func (p partitionedHashring) save() error {
	if p.store == nil {
		return nil
	}

	var data storeData
	if p.storeResources {
		data.Resources = p.GetAll()
	}
	data.Relations = p.relations
	return p.store.Save(data)
}
