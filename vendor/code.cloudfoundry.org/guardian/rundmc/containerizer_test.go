package rundmc_test

import (
	"bytes"
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/guardian/gardener"
	specpkg "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	fakes "code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Rundmc", func() {
	var (
		fakeDepot               *fakes.FakeDepot
		fakeBundleLoader        *fakes.FakeBundleLoader
		fakeOCIRuntime          *fakes.FakeOCIRuntime
		fakeNstarRunner         *fakes.FakeNstarRunner
		fakeStopper             *fakes.FakeStopper
		fakeEventStore          *fakes.FakeEventStore
		fakeStateStore          *fakes.FakeStateStore
		fakeRootfsFileCreator   *fakes.FakeRootfsFileCreator
		fakePeaCreator          *fakes.FakePeaCreator
		fakePeaUsernameResolver *fakes.FakePeaUsernameResolver

		logger        lager.Logger
		containerizer *rundmc.Containerizer
	)

	BeforeEach(func() {
		fakeDepot = new(fakes.FakeDepot)
		fakeOCIRuntime = new(fakes.FakeOCIRuntime)
		fakeBundleLoader = new(fakes.FakeBundleLoader)
		fakeNstarRunner = new(fakes.FakeNstarRunner)
		fakeStopper = new(fakes.FakeStopper)
		fakeEventStore = new(fakes.FakeEventStore)
		fakeStateStore = new(fakes.FakeStateStore)
		fakeRootfsFileCreator = new(fakes.FakeRootfsFileCreator)
		fakePeaCreator = new(fakes.FakePeaCreator)
		fakePeaUsernameResolver = new(fakes.FakePeaUsernameResolver)
		logger = lagertest.NewTestLogger("test")

		fakeDepot.LookupStub = func(_ lager.Logger, handle string) (string, error) {
			return "/path/to/" + handle, nil
		}

		containerizer = rundmc.New(fakeDepot, fakeOCIRuntime, fakeBundleLoader, fakeNstarRunner, fakeStopper, fakeEventStore, fakeStateStore, fakeRootfsFileCreator, fakePeaCreator, fakePeaUsernameResolver)
	})

	Describe("Create", func() {
		It("should ask the depot to create a container", func() {
			spec := specpkg.DesiredContainerSpec{
				Handle:     "exuberant!",
				BaseConfig: specs.Spec{Root: &specs.Root{}},
			}
			containerizer.Create(logger, spec)

			Expect(fakeDepot.CreateCallCount()).To(Equal(1))

			_, handle, actualSpec := fakeDepot.CreateArgsForCall(0)
			Expect(handle).To(Equal("exuberant!"))
			Expect(actualSpec).To(Equal(spec))
		})

		Context("when creating the depot directory fails", func() {
			It("returns an error", func() {
				fakeDepot.CreateReturns(errors.New("blam"))
				Expect(containerizer.Create(logger, specpkg.DesiredContainerSpec{
					Handle:     "exuberant!",
					BaseConfig: specs.Spec{Root: &specs.Root{}},
				})).NotTo(Succeed())
			})
		})

		It("should create a container in the given directory", func() {
			Expect(containerizer.Create(logger, specpkg.DesiredContainerSpec{
				Handle:     "exuberant!",
				BaseConfig: specs.Spec{Root: &specs.Root{}},
			})).To(Succeed())

			Expect(fakeOCIRuntime.CreateCallCount()).To(Equal(1))

			_, path, id, _ := fakeOCIRuntime.CreateArgsForCall(0)
			Expect(path).To(Equal("/path/to/exuberant!"))
			Expect(id).To(Equal("exuberant!"))
		})

		It("should prepare the root file system by creating mount points", func() {
			Expect(containerizer.Create(logger, specpkg.DesiredContainerSpec{
				Handle:     "exuberant!",
				BaseConfig: specs.Spec{Root: &specs.Root{Path: "some-rootfs"}},
			})).To(Succeed())

			Expect(fakeRootfsFileCreator.CreateFilesCallCount()).To(Equal(1))
			rootfsPath, createdFiles := fakeRootfsFileCreator.CreateFilesArgsForCall(0)
			Expect(rootfsPath).To(Equal("some-rootfs"))
			Expect(createdFiles).To(ConsistOf("/etc/hosts", "/etc/resolv.conf"))
		})

		Context("when creating files in the rootfs fails", func() {
			BeforeEach(func() {
				fakeRootfsFileCreator.CreateFilesReturns(errors.New("file-create-fail"))
			})

			It("returns the error", func() {
				Expect(containerizer.Create(logger, specpkg.DesiredContainerSpec{
					BaseConfig: specs.Spec{Root: &specs.Root{}},
				})).To(MatchError("file-create-fail"))
			})
		})

		Context("when the container creation fails", func() {
			BeforeEach(func() {
				fakeOCIRuntime.CreateReturns(errors.New("banana"))
			})

			It("should return an error", func() {
				Expect(containerizer.Create(logger, specpkg.DesiredContainerSpec{
					BaseConfig: specs.Spec{Root: &specs.Root{}},
				})).NotTo(Succeed())
			})
		})

		It("should watch for events in a goroutine", func() {
			fakeOCIRuntime.WatchEventsStub = func(_ lager.Logger, _ string, _ runrunc.EventsNotifier) error {
				time.Sleep(10 * time.Second)
				return nil
			}

			created := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				Expect(containerizer.Create(logger, specpkg.DesiredContainerSpec{
					Handle:     "some-container",
					BaseConfig: specs.Spec{Root: &specs.Root{}},
				})).To(Succeed())
				close(created)
			}()

			select {
			case <-time.After(2 * time.Second):
				Fail("WatchEvents should be called in a goroutine")
			case <-created:
			}

			Eventually(fakeOCIRuntime.WatchEventsCallCount).Should(Equal(1))

			_, handle, eventsNotifier := fakeOCIRuntime.WatchEventsArgsForCall(0)
			Expect(handle).To(Equal("some-container"))
			Expect(eventsNotifier).To(Equal(fakeEventStore))
		})
	})

	Describe("Run", func() {
		It("should ask the execer to exec a process in the container", func() {
			containerizer.Run(logger, "some-handle", garden.ProcessSpec{Path: "hello"}, garden.ProcessIO{})
			Expect(fakeOCIRuntime.ExecCallCount()).To(Equal(1))

			_, path, id, spec, _ := fakeOCIRuntime.ExecArgsForCall(0)
			Expect(path).To(Equal("/path/to/some-handle"))
			Expect(id).To(Equal("some-handle"))
			Expect(spec.Path).To(Equal("hello"))
		})

		Context("when process has no image", func() {
			It("doesn't create a pea", func() {
				containerizer.Run(logger, "some-handle", garden.ProcessSpec{Path: "hello"}, garden.ProcessIO{})
				Expect(fakePeaCreator.CreatePeaCallCount()).To(Equal(0))
			})

			Context("when bind mounts are provided", func() {
				It("returns an error", func() {
					_, err := containerizer.Run(logger, "some-handle",
						garden.ProcessSpec{
							Path: "hello",
							BindMounts: []garden.BindMount{
								garden.BindMount{
									SrcPath: "src",
									DstPath: "dst",
								},
							},
						},
						garden.ProcessIO{})
					Expect(err).To(MatchError("Running a process with bind mounts and no image provided is not allowed"))
				})
			})
		})

		Context("when process has an image", func() {
			var (
				processSpec garden.ProcessSpec
				pio         garden.ProcessIO
			)

			BeforeEach(func() {
				processSpec = garden.ProcessSpec{Image: garden.ImageRef{URI: "some-uri"}}
				pio = garden.ProcessIO{Stdout: bytes.NewBufferString("some-idiosyncratic buffer")}
			})

			It("creates a pea", func() {
				fakeDepot.LookupReturns("some-bundle-path", nil)
				containerizer.Run(logger, "some-handle", processSpec, pio)
				Expect(fakePeaCreator.CreatePeaCallCount()).To(Equal(1))
				_, actualProcessSpec, actualProcessIO, actualHandle, actualBundlePath := fakePeaCreator.CreatePeaArgsForCall(0)
				Expect(actualProcessSpec).To(Equal(processSpec))
				Expect(actualHandle).To(Equal("some-handle"))
				Expect(actualBundlePath).To(Equal("some-bundle-path"))
				Expect(actualProcessIO).To(Equal(pio))
			})

			It("returns process from pea creator", func() {
				fakeProcess := new(gardenfakes.FakeProcess)
				fakeProcess.IDReturns("some-id")
				fakePeaCreator.CreatePeaReturns(fakeProcess, nil)
				process, err := containerizer.Run(logger, "some-handle", processSpec, pio)
				Expect(process.ID()).To(Equal("some-id"))
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("Username resolving", func() {
				Context("when user is not specified", func() {
					It("does not try to resolve the user", func() {
						containerizer.Run(logger, "some-handle", processSpec, pio)
						Expect(fakePeaUsernameResolver.ResolveUserCallCount()).To(Equal(0))
					})
				})

				Context("when user is specified as uid:gid", func() {
					BeforeEach(func() {
						processSpec.User = "1:2"
					})

					It("does not try to resolve the user", func() {
						containerizer.Run(logger, "some-handle", processSpec, pio)
						Expect(fakePeaUsernameResolver.ResolveUserCallCount()).To(Equal(0))
					})
				})

				Context("when user is specified as username", func() {
					BeforeEach(func() {
						processSpec.User = "foobar"
					})

					It("resolves username to uid:gid", func() {
						fakePeaUsernameResolver.ResolveUserReturns(1, 2, nil)

						_, err := containerizer.Run(logger, "some-handle", processSpec, pio)
						Expect(err).NotTo(HaveOccurred())

						Expect(fakePeaUsernameResolver.ResolveUserCallCount()).To(Equal(1))
						_, _, _, _, resolverInputUsername := fakePeaUsernameResolver.ResolveUserArgsForCall(0)
						Expect(resolverInputUsername).To(Equal("foobar"))

						Expect(fakePeaCreator.CreatePeaCallCount()).To(Equal(1))
						_, createdPeaProcessSpec, _, _, _ := fakePeaCreator.CreatePeaArgsForCall(0)
						Expect(createdPeaProcessSpec.User).To(Equal("1:2"))
					})
				})
			})
		})

		Context("when looking up the container fails", func() {
			It("returns an error", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				_, err := containerizer.Run(logger, "some-handle", garden.ProcessSpec{}, garden.ProcessIO{})
				Expect(err).To(HaveOccurred())
			})

			It("does not attempt to exec the process", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				containerizer.Run(logger, "some-handle", garden.ProcessSpec{}, garden.ProcessIO{})
				Expect(fakeOCIRuntime.ExecCallCount()).To(Equal(0))
			})
		})
	})

	Describe("Attach", func() {
		It("should ask the execer to attach a process in the container", func() {
			containerizer.Attach(logger, "some-handle", "123", garden.ProcessIO{})
			Expect(fakeOCIRuntime.AttachCallCount()).To(Equal(1))

			_, path, id, processId, _ := fakeOCIRuntime.AttachArgsForCall(0)
			Expect(path).To(Equal("/path/to/some-handle"))
			Expect(id).To(Equal("some-handle"))
			Expect(processId).To(Equal("123"))
		})

		Context("when looking up the container fails", func() {
			It("returns an error", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				_, err := containerizer.Attach(logger, "some-handle", "123", garden.ProcessIO{})
				Expect(err).To(HaveOccurred())
			})

			It("does not attempt to exec the process", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				containerizer.Attach(logger, "some-handle", "123", garden.ProcessIO{})
				Expect(fakeOCIRuntime.AttachCallCount()).To(Equal(0))
			})
		})
	})

	Describe("StreamIn", func() {
		It("should execute the NSTar command with the container PID", func() {
			fakeOCIRuntime.StateReturns(runrunc.State{
				Pid: 12,
			}, nil)

			someStream := gbytes.NewBuffer()
			Expect(containerizer.StreamIn(logger, "some-handle", garden.StreamInSpec{
				Path:      "some-path",
				User:      "some-user",
				TarStream: someStream,
			})).To(Succeed())

			_, pid, path, user, stream := fakeNstarRunner.StreamInArgsForCall(0)
			Expect(pid).To(Equal(12))
			Expect(path).To(Equal("some-path"))
			Expect(user).To(Equal("some-user"))
			Expect(stream).To(Equal(someStream))
		})

		It("returns an error if the PID cannot be found", func() {
			fakeOCIRuntime.StateReturns(runrunc.State{}, errors.New("pid not found"))
			Expect(containerizer.StreamIn(logger, "some-handle", garden.StreamInSpec{})).To(MatchError("stream-in: pid not found for container"))
		})

		It("returns the error if nstar fails", func() {
			fakeNstarRunner.StreamInReturns(errors.New("failed"))
			Expect(containerizer.StreamIn(logger, "some-handle", garden.StreamInSpec{})).To(MatchError("stream-in: nstar: failed"))
		})
	})

	Describe("StreamOut", func() {
		It("should execute the NSTar command with the container PID", func() {
			fakeOCIRuntime.StateReturns(runrunc.State{
				Pid: 12,
			}, nil)

			fakeNstarRunner.StreamOutReturns(os.Stdin, nil)

			tarStream, err := containerizer.StreamOut(logger, "some-handle", garden.StreamOutSpec{
				Path: "some-path",
				User: "some-user",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(tarStream).To(Equal(os.Stdin))

			_, pid, path, user := fakeNstarRunner.StreamOutArgsForCall(0)
			Expect(pid).To(Equal(12))
			Expect(path).To(Equal("some-path"))
			Expect(user).To(Equal("some-user"))
		})

		It("returns an error if the PID cannot be found", func() {
			fakeOCIRuntime.StateReturns(runrunc.State{}, errors.New("pid not found"))
			tarStream, err := containerizer.StreamOut(logger, "some-handle", garden.StreamOutSpec{})

			Expect(tarStream).To(BeNil())
			Expect(err).To(MatchError("stream-out: pid not found for container"))
		})

		It("returns the error if nstar fails", func() {
			fakeNstarRunner.StreamOutReturns(nil, errors.New("failed"))
			tarStream, err := containerizer.StreamOut(logger, "some-handle", garden.StreamOutSpec{})

			Expect(tarStream).To(BeNil())
			Expect(err).To(MatchError("stream-out: nstar: failed"))
		})
	})

	Describe("Stop", func() {
		var (
			cgroupPathArg string
			exceptionsArg []int
			killArg       bool
		)

		Context("when the stop succeeds", func() {
			BeforeEach(func() {
				fakeOCIRuntime.StateReturns(runrunc.State{
					Pid: 1234,
				}, nil)

				Expect(containerizer.Stop(logger, "some-handle", true)).To(Succeed())
				Expect(fakeStopper.StopAllCallCount()).To(Equal(1))

				_, cgroupPathArg, exceptionsArg, killArg = fakeStopper.StopAllArgsForCall(0)
			})

			It("asks to stop all processes in the processes's cgroup", func() {
				Expect(cgroupPathArg).To(Equal("some-handle"))
				Expect(killArg).To(Equal(true))
			})

			It("asks to not stop the pid of the init process", func() {
				Expect(exceptionsArg).To(ConsistOf(1234))
			})

			It("transitions the stored state", func() {
				Expect(fakeStateStore.StoreStoppedCallCount()).To(Equal(1))
				handle := fakeStateStore.StoreStoppedArgsForCall(0)
				Expect(handle).To(Equal("some-handle"))
			})
		})

		Context("when the stop fails", func() {
			BeforeEach(func() {
				fakeStopper.StopAllReturns(errors.New("boom"))
			})

			It("does not transition to the stopped state", func() {
				Expect(containerizer.Stop(logger, "some-handle", true)).To(MatchError(ContainSubstring("boom")))
				Expect(fakeStateStore.StoreStoppedCallCount()).To(Equal(0))
			})
		})

		Context("when getting runc's state fails", func() {
			BeforeEach(func() {
				fakeOCIRuntime.StateReturns(runrunc.State{}, errors.New("boom"))
			})

			It("does not stop the processes", func() {
				Expect(fakeStopper.StopAllCallCount()).To(Equal(0))
			})

			It("does not transition to the stopped state", func() {
				Expect(containerizer.Stop(logger, "some-handle", true)).To(MatchError(ContainSubstring("boom")))
				Expect(fakeStateStore.StoreStoppedCallCount()).To(Equal(0))
			})
		})
	})

	Describe("Destroy", func() {
		Context("when getting state fails", func() {
			BeforeEach(func() {
				fakeOCIRuntime.StateReturns(runrunc.State{}, errors.New("pid not found"))
			})

			It("should NOT run delete", func() {
				Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
				Expect(fakeOCIRuntime.DeleteCallCount()).To(Equal(0))
			})
		})

		Context("when in a state that should result in a delete", func() {
			var status runrunc.Status

			stateThatShouldResultInADelete := func(force bool) {
				It("should run delete", func() {
					Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
					Expect(fakeOCIRuntime.DeleteCallCount()).To(Equal(1))
					_, passedForce, passedHandle := fakeOCIRuntime.DeleteArgsForCall(0)
					Expect(passedForce).To(Equal(force))
					Expect(passedHandle).To(Equal("some-handle"))
				})

				Context("when delete fails", func() {
					It("does not destroy the depot directory", func() {
						fakeOCIRuntime.DeleteReturns(errors.New("delete failed"))
						containerizer.Destroy(logger, "some-handle")
						Expect(fakeDepot.DestroyCallCount()).To(Equal(0))
					})
				})
			}

			JustBeforeEach(func() {
				fakeOCIRuntime.StateReturns(runrunc.State{
					Status: status,
				}, nil)
			})

			Context("when in the 'created' state", func() {
				BeforeEach(func() {
					status = "created"
				})

				stateThatShouldResultInADelete(false)
			})

			Context("when in the 'stopped' state", func() {
				BeforeEach(func() {
					status = "stopped"
				})

				stateThatShouldResultInADelete(false)
			})

			Context("when in the 'running' state", func() {
				BeforeEach(func() {
					status = "running"
				})

				stateThatShouldResultInADelete(true)
			})
		})

		Context("when state that should not result in a delete", func() {
			BeforeEach(func() {
				fakeOCIRuntime.StateReturns(runrunc.State{
					Status: "potato",
				}, nil)
			})

			It("should not run delete", func() {
				Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
				Expect(fakeOCIRuntime.DeleteCallCount()).To(Equal(0))
			})
		})
	})

	Describe("RemoveBundle", func() {
		It("removes the bundle from the depot", func() {
			Expect(containerizer.RemoveBundle(logger, "some-handle")).To(Succeed())
			Expect(fakeDepot.DestroyCallCount()).To(Equal(1))
			_, handle := fakeDepot.DestroyArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
		})

		Context("when removing bundle from depot fails", func() {
			BeforeEach(func() {
				fakeDepot.DestroyReturns(errors.New("destroy failed"))
			})

			It("returns an error", func() {
				Expect(containerizer.RemoveBundle(logger, "some-handle")).To(MatchError(ContainSubstring("destroy failed")))
			})
		})
	})

	Describe("Info", func() {
		var namespaces []specs.LinuxNamespace
		var resources *specs.LinuxResources

		BeforeEach(func() {
			namespaces = []specs.LinuxNamespace{}
			fakeOCIRuntime.StateReturns(runrunc.State{Pid: 42}, nil)

			var limit int64 = 10
			var shares uint64 = 20
			resources = &specs.LinuxResources{
				Memory: &specs.LinuxMemory{
					Limit: &limit,
				},
				CPU: &specs.LinuxCPU{
					Shares: &shares,
				},
			}

		})

		JustBeforeEach(func() {
			fakeBundleLoader.LoadStub = func(bundlePath string) (goci.Bndl, error) {
				if bundlePath != "/path/to/some-handle" {
					return goci.Bundle(), errors.New("cannot find bundle")
				}

				return goci.Bndl{
					Spec: specs.Spec{
						Root: &specs.Root{},
						Linux: &specs.Linux{
							Namespaces: namespaces,
							Resources:  resources,
						},
					},
				}, nil
			}
		})

		It("should return the ActualContainerSpec with the correct bundlePath", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.BundlePath).To(Equal("/path/to/some-handle"))
		})

		It("should return the ActualContainerSpec with the correct CPU limits", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.Limits.CPU.LimitInShares).To(BeEquivalentTo(20))
		})

		It("should return the ActualContainerSpec with the correct memory limits", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.Limits.Memory.LimitInBytes).To(BeEquivalentTo(10))
		})

		It("should return the ActualContainerSpec with the correct pid", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.Pid).To(Equal(42))
		})

		Context("when looking up the bundle path fails", func() {
			It("should return the error", func() {
				fakeDepot.LookupReturns("", errors.New("spiderman-error"))
				_, err := containerizer.Info(logger, "some-handle")
				Expect(err).To(MatchError("spiderman-error"))
			})
		})

		Context("when the bundle has no resources", func() {
			BeforeEach(func() {
				resources = nil
			})

			It("doesn't fail", func() {
				actualSpec, err := containerizer.Info(logger, "some-handle")
				Expect(err).NotTo(HaveOccurred())
				Expect(actualSpec.Limits.Memory.LimitInBytes).To(BeEquivalentTo(0))
				Expect(actualSpec.Limits.CPU.LimitInShares).To(BeEquivalentTo(0))
			})
		})

		Context("when loading the bundle fails", func() {
			It("should return the error", func() {
				fakeBundleLoader.LoadReturns(goci.Bundle(), errors.New("aquaman-error"))
				_, err := containerizer.Info(logger, "some-handle")
				Expect(err).To(MatchError("aquaman-error"))
			})
		})

		It("should return any events from the event store", func() {
			fakeEventStore.EventsReturns([]string{
				"potato",
				"fire",
			})

			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.Events).To(Equal([]string{
				"potato",
				"fire",
			}))
		})

		It("should return the stopped state from the property manager", func() {
			fakeStateStore.IsStoppedReturns(true)

			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.Stopped).To(Equal(true))
		})

		It("should return the ActualContainerSpec with privileged by default", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.Privileged).To(BeTrue())
		})

		Context("when namespaces contains the user namespace", func() {
			BeforeEach(func() {
				namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.UserNamespace})
			})

			It("should return the ActualContainerSpec with privileged", func() {
				actualSpec, err := containerizer.Info(logger, "some-handle")
				Expect(err).NotTo(HaveOccurred())
				Expect(actualSpec.Privileged).To(BeFalse())
			})
		})

		Context("when retrieving the State from the runtime errors", func() {
			BeforeEach(func() {
				fakeOCIRuntime.StateReturns(runrunc.State{}, errors.New("error-fetching-state"))
			})

			It("returns the error", func() {
				_, err := containerizer.Info(logger, "some-handle")
				Expect(err).To(MatchError("error-fetching-state"))
			})
		})
	})

	Describe("Metrics", func() {
		It("returns the CPU metrics", func() {
			metrics := gardener.ActualContainerMetrics{
				CPU: garden.ContainerCPUStat{
					Usage:  1,
					User:   2,
					System: 3,
				},
			}

			fakeOCIRuntime.StatsReturns(metrics, nil)
			Expect(containerizer.Metrics(logger, "foo")).To(Equal(metrics))
		})

		Context("when container fails to provide stats", func() {
			BeforeEach(func() {
				fakeOCIRuntime.StatsReturns(gardener.ActualContainerMetrics{}, errors.New("banana"))
			})

			It("should return the error", func() {
				_, err := containerizer.Metrics(logger, "foo")
				Expect(err).To(MatchError("banana"))
			})
		})
	})

	Describe("handles", func() {
		Context("when handles exist", func() {
			BeforeEach(func() {
				fakeDepot.HandlesReturns([]string{"banana", "banana2"}, nil)
			})

			It("should return the handles", func() {
				Expect(containerizer.Handles()).To(ConsistOf("banana", "banana2"))
			})
		})

		Context("when the depot returns an error", func() {
			testErr := errors.New("spiderman error")

			BeforeEach(func() {
				fakeDepot.HandlesReturns([]string{}, testErr)
			})

			It("should return the error", func() {
				_, err := containerizer.Handles()
				Expect(err).To(MatchError(testErr))
			})
		})
	})
})
