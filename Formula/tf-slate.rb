class TfSlate < Formula
  desc "Discover, summarize, and manage local Terraform state files"
  homepage "https://github.com/abuxton/tf-slate"
  license "MIT"
  head "https://github.com/abuxton/tf-slate.git", branch: "main" do
    depends_on "go" => :build
  end

  on_macos do
    on_arm do
      url "https://github.com/abuxton/tf-slate/releases/download/v0.3.0/tf-slate_0.3.0_darwin_arm64.tar.gz"
      sha256 "c99ccbaaa66468f292fcefb4fd863a5fcec968ba280d751a926d5175d96b67b1"
    end
    on_intel do
      url "https://github.com/abuxton/tf-slate/releases/download/v0.3.0/tf-slate_0.3.0_darwin_amd64.tar.gz"
      sha256 "e8db32d072abd2f667beff74d5c409b5fe40baaa84014f915e198a1ef49066dc"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/abuxton/tf-slate/releases/download/v0.3.0/tf-slate_0.3.0_linux_arm64.tar.gz"
      sha256 "e9883c1039637c96a464d5c6edea196caae6947be0f56df004dab451e3adfc6d"
    end
    on_intel do
      url "https://github.com/abuxton/tf-slate/releases/download/v0.3.0/tf-slate_0.3.0_linux_amd64.tar.gz"
      sha256 "33dee34fd7bf128cffea16073ffd9329de176ce7368435d2d65af2df85ecbd08"
    end
  end

  def install
    if build.head?
      system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=HEAD"), "./cmd/tf-slate"
    else
      bin.install "tf-slate"
    end
  end

  def caveats
    <<~EOS
      Terraform is optional. It is only needed for the interactive follow-up
      commands (list, show, destroy); scanning and non-interactive output work without it.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tf-slate --version")
    assert_match "No Terraform state files found", shell_output("#{bin}/tf-slate --ni #{testpath}")
  end
end
