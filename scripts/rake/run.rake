BACKEND_DIR = 'backend'

namespace :run do

  desc 'run backend'
  task :backend do
    system %{
      cd "#{BACKEND_DIR}"; go run -race ./cmd/secretdrop/
    }
  rescue Interrupt
    exit(130)
  end

end