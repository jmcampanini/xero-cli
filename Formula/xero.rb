class Xero < Formula
  desc "Inspect Xero organisations, accounts, and tracking from the terminal"
  homepage "https://github.com/jmcampanini/xero-cli"
  license "MIT"
  head "https://github.com/jmcampanini/xero-cli.git", branch: "main"

  depends_on "go" => :build

  def install
    identity = Utils.safe_popen_read("git", "describe", "--tags", "--dirty", "--always").strip
    odie "Cannot determine build identity" if identity.empty?
    ldflags = "-X github.com/jmcampanini/xero-cli/cmd.Version=#{identity}"
    system "go", "build", "-buildvcs=false", *std_go_args(ldflags:)
    generate_completions_from_executable(bin/"xero", "completion")
  end

  test do
    assert_match "xero version ", shell_output("#{bin}/xero --version")
    assert_match "xero exits 0, 1, or 2.", shell_output("#{bin}/xero exit-codes")
  end
end
