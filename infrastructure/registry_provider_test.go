package infrastructure_test

import (
	"errors"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"

	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("RegistryProvider", func() {
	var (
		metadataService  *fakeinf.FakeMetadataService
		platform         *platformfakes.FakePlatform
		fs               *fakesys.FakeFileSystem
		registryProvider RegistryProvider
		logger           boshlog.Logger
	)

	BeforeEach(func() {
		metadataService = &fakeinf.FakeMetadataService{}
		platform = &platformfakes.FakePlatform{}
		fs = fakesys.NewFakeFileSystem()
	})

	JustBeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)
		registryProvider = NewRegistryProvider(metadataService, platform, fs, logger)
	})

	Describe("GetRegistry", func() {
		Context("when metadata service returns registry http endpoint", func() {
			BeforeEach(func() {
				metadataService.RegistryEndpoint = "http://registry-endpoint"
			})

			Context("when registry is configured to not use server name as id", func() {

				It("returns an http registry that does not use server name as id", func() {
					registry, err := registryProvider.GetRegistry()
					Expect(err).ToNot(HaveOccurred())
					Expect(registry).To(Equal(NewHTTPRegistry(metadataService, platform, false, logger)))
				})
			})

			Context("when registry is configured to use server name as id", func() {

				It("returns an http registry that uses server name as id", func() {
					registry, err := registryProvider.GetRegistry()
					Expect(err).ToNot(HaveOccurred())
					Expect(registry).To(Equal(NewHTTPRegistry(metadataService, platform, true, logger)))
				})
			})
		})

		Context("when metadata service returns registry file endpoint", func() {
			BeforeEach(func() {
				metadataService.RegistryEndpoint = "/tmp/registry-endpoint"
			})

			It("returns a file registry", func() {
				registry, err := registryProvider.GetRegistry()
				Expect(err).ToNot(HaveOccurred())
				Expect(registry).To(Equal(NewFileRegistry("/tmp/registry-endpoint", fs)))
			})
		})

		Context("when metadata service returns an error", func() {
			BeforeEach(func() {
				metadataService.GetRegistryEndpointErr = errors.New("fake-get-registry-endpoint-error")
			})

			It("returns error", func() {
				_, err := registryProvider.GetRegistry()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-registry-endpoint-error"))
			})
		})
	})
})
