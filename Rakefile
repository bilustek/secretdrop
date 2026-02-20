# frozen_string_literal: true

require 'English'
require 'pathname'

$LOAD_PATH << './scripts/rake'

Dir.glob('scripts/rake/**/*.rake').each { |r| import r }

task :default => ['run:backend']


