BACKEND_DIR = 'backend'
FRONTEND_DIR = 'frontend'
STRIPE_WEBHOOK_FORWARD_TO = ENV['STRIPE_WEBHOOK_FORWARD_TO'] || 'localhost:8080/billing/webhook'

namespace :run do

  desc 'run backend'
  task :backend do
    begin
      pid_backend = %{ cd "#{BACKEND_DIR}"; go run -race ./cmd/secretdrop/ }
      pid_frontend = %{ cd "#{FRONTEND_DIR}"; npm run dev }
      pid_stripe_local_webhook = %{ stripe listen --forward-to #{STRIPE_WEBHOOK_FORWARD_TO} }
      pids = [
        Process.spawn(pid_backend),
        Process.spawn(pid_frontend),
        Process.spawn(pid_stripe_local_webhook),
      ]

      Signal.trap('INT') do # 2
        puts '-> signal received, attempting graceful shutdown...'

        pids.each do |pid|
          Process.kill('TERM', pid) # 15
        rescue Errno::ESRCH
          next
        end

        sleep 1

        pids.each do |pid|
          Process.getpgid(pid) # check if the process is still running
          Process.kill('KILL', pid) # 9
        rescue Errno::ESRCH
          next
        end

        puts '-> all clear, exiting...'
        exit($CHILD_STATUS.nil? ? 1 : $CHILD_STATUS.exitstatus) unless ENV['RAKE_CONTINUE']
      end

      pids.each { |pid| Process.wait(pid) }
    rescue Interrupt
      puts 'rake handled interruption'
    ensure
      exit($CHILD_STATUS.nil? ? 1 : $CHILD_STATUS.exitstatus) unless ENV['RAKE_CONTINUE']
    end

    exit $CHILD_STATUS.exitstatus unless ENV['RAKE_CONTINUE']
  end
end