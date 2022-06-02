package infrastructure

import (
	boshplat "github.com/cloudfoundry/bosh-agent/platform"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type RegistryProvider interface {
	GetRegistry() (Registry, error)
}

type registryProvider struct {
	metadataService MetadataService
	platform        boshplat.Platform
	fs              boshsys.FileSystem
	logTag          string
	logger          boshlog.Logger
}

func NewRegistryProvider(
	metadataService MetadataService,
	platform boshplat.Platform,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) RegistryProvider {
	return &registryProvider{
		metadataService: metadataService,
		platform:        platform,
		fs:              fs,
		logTag:          "registryProvider",
		logger:          logger,
	}
}

func (p *registryProvider) GetRegistry() (Registry, error) {

	p.logger.Debug(p.logTag, "Using file registry at %s", "registryEndpoint")
	return NewFileRegistry("registryEndpoint", p.fs), nil
}
