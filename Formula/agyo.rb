class Agyo < Formula
  desc "Autonomous Session Agent Engine & Outer Harness for Google Antigravity"
  homepage "https://github.com/tiagovilasboas/antigravity-operator"
  url "https://github.com/tiagovilasboas/antigravity-operator/archive/refs/tags/v0.4.5.tar.gz"
  sha256 "b6d9e68717fa064fdca5ad2dbb7cb088bf44779f4f742ff3f79cd82414be0f7e"
  license "MIT"
  head "https://github.com/tiagovilasboas/antigravity-operator.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", "-ldflags", "-s -w -X main.Version=#{version}", "-o", bin/"agyo", "./cmd/agyo"
    bin.install_symlink bin/"agyo" => "antigravity-operator"

    generate_completions_from_executable(bin/"agyo", "completion")
  end

  test do
    assert_match "agyo (Antigravity Operator) v#{version}", shell_output("#{bin}/agyo version")
    assert_match "Antigravity Operator Doctor", shell_output("#{bin}/agyo doctor")
  end
end
