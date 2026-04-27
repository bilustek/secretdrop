# frozen_string_literal: true
require 'English'

namespace :install do
  desc 'install frontend dependencies'
  task :frontend do
    system %{ cd "#{FRONTEND_DIR}"; npm install }
    exit($CHILD_STATUS&.exitstatus || 1) unless ENV['RAKE_CONTINUE']
  rescue Interrupt
    exit(130)
  end
end