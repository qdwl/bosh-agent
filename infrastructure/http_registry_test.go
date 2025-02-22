package infrastructure_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"

	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("httpRegistry", describeHTTPRegistry)

func describeHTTPRegistry() { //nolint:funlen
	logger := boshlog.NewLogger(boshlog.LevelNone)

	var (
		metadataService *fakeinf.FakeMetadataService
		registry        Registry
		platform        *platformfakes.FakePlatform
	)

	BeforeEach(func() {
		metadataService = &fakeinf.FakeMetadataService{}
		platform = &platformfakes.FakePlatform{}
		registry = NewHTTPRegistry(metadataService, platform, false, logger)
	})

	Describe("GetSettings", func() {
		var (
			ts           *httptest.Server
			settingsJSON string
		)

		Context("when server responds successfully to the first request", func() {
			BeforeEach(func() {
				boshRegistryHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					GinkgoRecover()

					Expect(r.Method).To(Equal("GET"))
					Expect(r.URL.Path).To(Equal("/instances/fake-identifier/settings"))

					_, err := w.Write([]byte(settingsJSON))
					Expect(err).NotTo(HaveOccurred())
				})

				ts = httptest.NewServer(boshRegistryHandler)
			})

			AfterEach(func() {
				ts.Close()
			})

			Describe("Network bootstrapping", func() {
				BeforeEach(func() {
					settingsJSON = `{"settings": "{\"agent_id\":\"my-agent-id\"}"}`
					metadataService.InstanceID = "fake-identifier"
					metadataService.RegistryEndpoint = ts.URL
					registry = NewHTTPRegistry(metadataService, platform, false, logger)
				})

				Context("when the metadata has Networks information", func() {
					It("configures the network with those settings before hitting the registry", func() {
						networkSettings := boshsettings.Networks{
							"net1": boshsettings.Network{IP: "1.2.3.4"},
							"net2": boshsettings.Network{IP: "2.3.4.5"},
						}
						metadataService.Networks = networkSettings

						_, err := registry.GetSettings()
						Expect(err).ToNot(HaveOccurred())

						Expect(platform.SetupNetworkingCallCount()).To(Equal(1))
						Expect(platform.SetupNetworkingArgsForCall(0)).To(Equal(networkSettings))
					})
				})

				Context("when the metadata has no Networks information", func() {
					It("does no network configuration for now (the stemcell set up dhcp already)", func() {
						metadataService.Networks = boshsettings.Networks{}

						_, err := registry.GetSettings()
						Expect(err).ToNot(HaveOccurred())

						Expect(platform.SetupNetworkingCallCount()).To(Equal(0))
					})
				})

				Context("when the metadata service fails to get Networks information", func() {
					It("wraps the error", func() {
						metadataService.Networks = boshsettings.Networks{}
						metadataService.NetworksErr = errors.New("fake-get-networks-err")

						_, err := registry.GetSettings()
						Expect(err).To(HaveOccurred())

						Expect(err.Error()).To(Equal("Getting networks: fake-get-networks-err"))
					})
				})

				Context("when the SetupNetworking fails", func() {
					It("wraps the error", func() {
						networkSettings := boshsettings.Networks{
							"net1": boshsettings.Network{IP: "1.2.3.4"},
							"net2": boshsettings.Network{IP: "2.3.4.5"},
						}
						metadataService.Networks = networkSettings
						platform.SetupNetworkingReturns(errors.New("fake-setup-networking-error"))

						_, err := registry.GetSettings()
						Expect(err).To(HaveOccurred())

						Expect(err.Error()).To(Equal("Setting up networks: fake-setup-networking-error"))
					})
				})
			})

			Context("when registry is configured to not use server name as id", func() {
				BeforeEach(func() {
					registry = NewHTTPRegistry(metadataService, platform, false, logger)
					metadataService.InstanceID = "fake-identifier"
					metadataService.RegistryEndpoint = ts.URL
				})

				It("returns settings fetched from http server based on instance id", func() {
					settingsJSON = `{"settings": "{\"agent_id\":\"my-agent-id\"}"}`

					settings, err := registry.GetSettings()
					Expect(err).ToNot(HaveOccurred())
					Expect(settings).To(Equal(boshsettings.Settings{AgentID: "my-agent-id"}))
				})

				It("returns error if registry settings wrapper cannot be parsed", func() {
					settingsJSON = "invalid-json"

					settings, err := registry.GetSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Unmarshalling settings wrapper"))

					Expect(settings).To(Equal(boshsettings.Settings{}))
				})

				It("returns error if registry settings wrapper contains invalid json", func() {
					settingsJSON = `{"settings": "invalid-json"}`

					settings, err := registry.GetSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Unmarshalling wrapped settings"))

					Expect(settings).To(Equal(boshsettings.Settings{}))
				})

				It("returns error if metadata service fails to return instance id", func() {
					metadataService.GetInstanceIDErr = errors.New("fake-get-instance-id-err")

					settings, err := registry.GetSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-get-instance-id-err"))

					Expect(settings).To(Equal(boshsettings.Settings{}))
				})

				It("returns error if metadata service fails to return registry endpoint", func() {
					metadataService.GetRegistryEndpointErr = errors.New("fake-get-registry-endpoint-err")

					settings, err := registry.GetSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-get-registry-endpoint-err"))

					Expect(settings).To(Equal(boshsettings.Settings{}))
				})

				Describe("setting fields", func() {
					It("unmarshalls JSON properly", func() {
						settingsJSON = `{
						"agent_id": "my-agent-id",
						"blobstore": {
							"options": {
								"bucket_name": "george",
								"encryption_key": "optional encryption key",
								"access_key_id": "optional access key id",
								"secret_access_key": "optional secret access key",
								"port": 443
							},
							"provider": "s3"
						},
						"disks": {
							"ephemeral": "/dev/sdb",
							"persistent": {
								"vol-xxxxxx": "/dev/sdf"
							},
							"system": "/dev/sda1"
						},
						"env": {
							"bosh": {
								"password": "some encrypted password",
								"keep_root_password": false
							}
						},
						"networks": {
							"netA": {
								"default": ["dns", "gateway"],
								"ip": "ww.ww.ww.ww",
								"dns": [
									"xx.xx.xx.xx",
									"yy.yy.yy.yy"
								]
							},
							"netB": {
								"dns": [
									"zz.zz.zz.zz"
								]
							}
						},
						"mbus": "https://vcap:b00tstrap@0.0.0.0:6868",
						"ntp": [
							"0.north-america.pool.ntp.org",
							"1.north-america.pool.ntp.org"
						],
						"vm": {
							"name": "vm-abc-def"
						}
					}`
						settingsJSON = strings.ReplaceAll(settingsJSON, `"`, `\"`)
						settingsJSON = strings.ReplaceAll(settingsJSON, "\n", "")
						settingsJSON = strings.ReplaceAll(settingsJSON, "\t", "")
						settingsJSON = fmt.Sprintf(`{"settings": "%s"}`, settingsJSON)

						expectedSettings := boshsettings.Settings{
							AgentID: "my-agent-id",
							Blobstore: boshsettings.Blobstore{
								Type: "s3",
								Options: map[string]interface{}{
									"bucket_name":       "george",
									"encryption_key":    "optional encryption key",
									"access_key_id":     "optional access key id",
									"secret_access_key": "optional secret access key",
									"port":              443.0,
								},
							},
							Disks: boshsettings.Disks{
								Ephemeral:  "/dev/sdb",
								Persistent: map[string]interface{}{"vol-xxxxxx": "/dev/sdf"},
								System:     "/dev/sda1",
							},
							Env: boshsettings.Env{
								Bosh: boshsettings.BoshEnv{
									Password:         "some encrypted password",
									KeepRootPassword: false,
								},
							},
							Networks: boshsettings.Networks{
								"netA": boshsettings.Network{
									Default: []string{"dns", "gateway"},
									IP:      "ww.ww.ww.ww",
									DNS:     []string{"xx.xx.xx.xx", "yy.yy.yy.yy"},
								},
								"netB": boshsettings.Network{
									DNS: []string{"zz.zz.zz.zz"},
								},
							},
							Mbus: "https://vcap:b00tstrap@0.0.0.0:6868",
							NTP: []string{
								"0.north-america.pool.ntp.org",
								"1.north-america.pool.ntp.org",
							},
							VM: boshsettings.VM{
								Name: "vm-abc-def",
							},
						}

						metadataService.InstanceID = "fake-identifier"
						metadataService.RegistryEndpoint = ts.URL

						settings, err := registry.GetSettings()
						Expect(err).ToNot(HaveOccurred())
						Expect(settings).To(Equal(expectedSettings))
					})
				})
			})

			Context("when registry is configured to use server name as id", func() {
				BeforeEach(func() {
					registry = NewHTTPRegistry(metadataService, platform, true, logger)
					metadataService.ServerName = "fake-identifier"
					metadataService.RegistryEndpoint = ts.URL
				})

				It("returns settings fetched from http server based on server name", func() {
					settingsJSON = `{"settings": "{\"agent_id\":\"my-agent-id\"}"}`

					settings, err := registry.GetSettings()
					Expect(err).ToNot(HaveOccurred())
					Expect(settings).To(Equal(boshsettings.Settings{AgentID: "my-agent-id"}))
				})

				It("returns error if registry settings wrapper cannot be parsed", func() {
					settingsJSON = "invalid-json"

					settings, err := registry.GetSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Unmarshalling settings wrapper"))

					Expect(settings).To(Equal(boshsettings.Settings{}))
				})

				It("returns error if registry settings wrapper contains invalid json", func() {
					settingsJSON = `{"settings": "invalid-json"}`

					settings, err := registry.GetSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Unmarshalling wrapped settings"))

					Expect(settings).To(Equal(boshsettings.Settings{}))
				})

				It("returns error if metadata service fails to return server name", func() {
					metadataService.GetServerNameErr = errors.New("fake-get-server-name-err")

					settings, err := registry.GetSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-get-server-name-err"))

					Expect(settings).To(Equal(boshsettings.Settings{}))
				})

				It("returns error if metadata service fails to return registry endpoint", func() {
					metadataService.GetRegistryEndpointErr = errors.New("fake-get-registry-endpoint-err")

					settings, err := registry.GetSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-get-registry-endpoint-err"))

					Expect(settings).To(Equal(boshsettings.Settings{}))
				})
			})
		})

		Context("when server does not respond successfully to the first request", func() {
			successfulAfter := func(count int) func(http.ResponseWriter, *http.Request) {
				initialCount := 0
				return func(w http.ResponseWriter, r *http.Request) {
					if initialCount < count {
						initialCount++
						http.Error(w, http.StatusText(500), 500)
						return
					}

					GinkgoRecover()

					_, err := w.Write([]byte(settingsJSON))
					Expect(err).NotTo(HaveOccurred())
				}
			}

			var handlerFunc http.Handler

			JustBeforeEach(func() {
				settingsJSON = `{"settings": "{\"agent_id\":\"my-agent-id\"}"}`
				ts = httptest.NewServer(handlerFunc)
				registry = NewHTTPRegistryWithCustomDelay(metadataService, platform, false, logger, 0*time.Second)
				metadataService.InstanceID = "fake-identifier"
				metadataService.RegistryEndpoint = ts.URL
			})

			AfterEach(func() {
				ts.Close()
			})

			Context("when server responds successfully within 10 retries", func() {
				BeforeEach(func() {
					handlerFunc = http.HandlerFunc(successfulAfter(9))
				})

				It("returns settings fetched from http server", func() {
					settings, err := registry.GetSettings()
					Expect(err).ToNot(HaveOccurred())
					Expect(settings).To(Equal(boshsettings.Settings{AgentID: "my-agent-id"}))
				})
			})

			Context("when server does NOT respond successfully within 10 retries", func() {
				BeforeEach(func() {
					handlerFunc = http.HandlerFunc(successfulAfter(10))
				})

				It("returns settings fetched from http server", func() {
					_, err := registry.GetSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid status: 500"))
				})
			})
		})
	})
}
