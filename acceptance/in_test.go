package acceptance

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf-experimental/pivnet-resource/concourse"
)

var _ = Describe("In", func() {
	var (
		productVersion = "pivnet-testing"
		destDirectory  string

		command       *exec.Cmd
		inRequest     concourse.InRequest
		stdinContents []byte
	)

	BeforeEach(func() {
		var err error

		By("Creating temp directory")
		destDirectory, err = ioutil.TempDir("", "pivnet-resource")
		Expect(err).NotTo(HaveOccurred())

		By("Creating command object")
		command = exec.Command(inPath, destDirectory)

		By("Creating default request")
		inRequest = concourse.InRequest{
			Source: concourse.Source{
				APIToken:    pivnetAPIToken,
				ProductSlug: productSlug,
				Endpoint:    endpoint,
			},
			Version: concourse.Version{
				ProductVersion: productVersion,
			},
		}

		stdinContents, err = json.Marshal(inRequest)
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		By("Removing temporary destination directory")
		err := os.RemoveAll(destDirectory)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns valid json", func() {
		By("Running the command")
		session := run(command, stdinContents)
		Eventually(session, executableTimeout).Should(gexec.Exit(0))

		By("Outputting a valid json response")
		response := concourse.InResponse{}
		err := json.Unmarshal(session.Out.Contents(), &response)
		Expect(err).ShouldNot(HaveOccurred())

		By("Validating output contains correct product version")
		Expect(response.Version.ProductVersion).To(Equal(productVersion))

		By("Validing the returned metadata is present")
		_, err = metadataValueForKey(response.Metadata, "release_type")
		Expect(err).ShouldNot(HaveOccurred())

		_, err = metadataValueForKey(response.Metadata, "release_date")
		Expect(err).ShouldNot(HaveOccurred())

		_, err = metadataValueForKey(response.Metadata, "description")
		Expect(err).ShouldNot(HaveOccurred())

		_, err = metadataValueForKey(response.Metadata, "release_notes_url")
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("does not download any of the files in the specified release", func() {
		By("Running the command")
		session := run(command, stdinContents)
		Eventually(session, executableTimeout).Should(gexec.Exit(0))

		By("Reading downloaded files")
		dataDir, err := os.Open(destDirectory)
		Expect(err).ShouldNot(HaveOccurred())

		By("Validating number of downloaded files is zero")
		_, err = dataDir.Readdir(0)
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("creates a version file with the downloaded version", func() {
		versionFilepath := filepath.Join(destDirectory, "version")

		By("Running the command")
		session := run(command, stdinContents)
		Eventually(session, executableTimeout).Should(gexec.Exit(0))

		By("Validating version file has correct contents")
		contents, err := ioutil.ReadFile(versionFilepath)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(contents)).To(Equal(productVersion))
	})

	Context("when globs are provided", func() {
		It("downloads only the files that match the glob", func() {
			By("setting the glob")
			inRequest.Source.ProductSlug = "pivnet-resource-test"
			inRequest.Version.ProductVersion = "0.0.0"
			inRequest.Params.Globs = []string{"*.jpg"}

			globStdInRequest, err := json.Marshal(inRequest)
			Expect(err).ShouldNot(HaveOccurred())

			By("Running the command")
			session := run(command, globStdInRequest)
			Eventually(session, executableTimeout).Should(gexec.Exit(0))

			By("Reading downloaded files")
			dataDir, err := os.Open(destDirectory)
			Expect(err).ShouldNot(HaveOccurred())

			By("Validating number of downloaded files")
			files, err := dataDir.Readdir(2)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(files).To(HaveLen(2))

			By("Validating files have non-zero-length content")
			for _, f := range files {
				if f.Name() == "5375828702_2832ae812c_b.jpg" {
					Expect(f.Size()).To(Equal(int64(329795)))
				}
			}
		})
	})

	Context("when two globs are provided", func() {
		Context("when one glob matches and one does not", func() {
			It("should see a job error", func() {
				By("setting the glob")
				inRequest.Source.ProductSlug = "pivnet-resource-test"
				inRequest.Version.ProductVersion = "0.0.0"
				inRequest.Params.Globs = []string{"*.jpg", "*.txt"}

				globStdInRequest, err := json.Marshal(inRequest)
				Expect(err).ShouldNot(HaveOccurred())

				By("Running the command")
				session := run(command, globStdInRequest)
				Eventually(session, executableTimeout).Should(gexec.Exit(1))

				By("Verifying stderr of command")
				Eventually(session.Err).Should(gbytes.Say("Failed to filter Product Files: no files match glob: "))
			})
		})
	})
})
