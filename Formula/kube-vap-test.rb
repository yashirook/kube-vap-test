class KubeVapTest < Formula
  desc "ValidatingAdmissionPolicy Test Tool for Kubernetes"
  homepage "https://github.com/yashirook/kube-vap-test"
  url "https://github.com/yashirook/kube-vap-test/archive/v1.31.0.tar.gz"
  sha256 "PLACEHOLDER_SHA256"
  license "MIT"
  head "https://github.com/yashirook/kube-vap-test.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/yashirook/kube-vap-test/cmd/kube-vap-test/commands.Version=#{version}
      -X github.com/yashirook/kube-vap-test/cmd/kube-vap-test/commands.Commit=#{tap.user}
      -X github.com/yashirook/kube-vap-test/cmd/kube-vap-test/commands.BuildDate=#{time.iso8601}
    ]

    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/kube-vap-test"

    # Install shell completions if supported
    generate_completions_from_executable(bin/"kube-vap-test", "completion")
  end

  test do
    # Test basic functionality
    assert_match version.to_s, shell_output("#{bin}/kube-vap-test version --short")
    
    # Test help command
    assert_match "ValidatingAdmissionPolicy Test Tool", shell_output("#{bin}/kube-vap-test --help")
  end
end