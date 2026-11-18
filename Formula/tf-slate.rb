class TfSlate < Formula
  desc "Discover, summarize, and manage local Terraform state files"
  homepage "https://github.com/abuxton/tf-slate"
  url "https://github.com/abuxton/tf-slate/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "db3ce68a5f1866c1e6d60da206ae6fa05e9fa75c2e22dd2f5db9a958ee8445a1"
  license "MIT"
  head "https://github.com/abuxton/tf-slate.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=v#{version}"), "./cmd/tf-slate"
  end

  def caveats
    <<~EOS
      Terraform is optional. It is only needed for the interactive follow-up
      commands (list, show, destroy); scanning and non-interactive output work without it.
    EOS
  end

  test do
    assert_match "v#{version}", shell_output("#{bin}/tf-slate --version")
    assert_match "No Terraform state files found", shell_output("#{bin}/tf-slate --ni #{testpath}")
  end
end
