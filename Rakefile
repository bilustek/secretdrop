# frozen_string_literal: true

require 'English'
require 'pathname'

$LOAD_PATH << './scripts/rake'

Dir.glob('scripts/rake/**/*.rake').each { |r| import r }

BACKEND_DIR = 'backend'
FRONTEND_DIR = 'frontend'
STRIPE_WEBHOOK_FORWARD_TO = ENV['STRIPE_WEBHOOK_FORWARD_TO'] || 'localhost:8080/billing/webhook'

task :default => ['run:backend']


