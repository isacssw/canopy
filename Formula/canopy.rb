# typed: false
# frozen_string_literal: true

# This formula is auto-updated by goreleaser on release.
# To use before the homebrew-tap repo is set up, copy this file to
# isacssw/homebrew-tap/Formula/canopy.rb on GitHub.

class Canopy < Formula
  desc "Terminal UI for managing parallel AI coding agents"
  homepage "https://github.com/isacssw/canopy"
  license "MIT"

  # goreleaser replaces this block on every release
  on_macos do
    on_arm do
      url "https://github.com/isacssw/canopy/releases/download/v#{version}/canopy_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    end
    on_intel do
      url "https://github.com/isacssw/canopy/releases/download/v#{version}/canopy_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/isacssw/canopy/releases/download/v#{version}/canopy_linux_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    end
    on_intel do
      url "https://github.com/isacssw/canopy/releases/download/v#{version}/canopy_linux_amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  depends_on "tmux"

  def install
    bin.install "canopy"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/canopy --version")
  end
end
