package fixtures

import "fmt"

func RunRSyncScript() string {
	return fmt.Sprintf(runnerScript, "\n\trsync -a /tmp/local/ /home/vcap/app/")
}

func CommitScript() string {
	return fmt.Sprintf(runnerScript, "")
}

func ForwardScript() string {
	return forwardScript
}

const forwardScript = `
	echo 'Forwarding: some-name some-other-name'
	sshpass -f /tmp/ssh-code ssh -4 -N \
	    -o PermitLocalCommand=yes -o LocalCommand="touch /tmp/healthy" \
		-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no \
		-o LogLevel=ERROR -o ExitOnForwardFailure=yes \
		-o ServerAliveInterval=10 -o ServerAliveCountMax=60 \
		-p 'some-port' 'some-user@some-ssh-host' \
		-L 'some-from:some-to' \
		-L 'some-other-from:some-other-to'
	rm -f /tmp/healthy
`

const runnerScript = `
	set -e%s
	exec /launcher "$1"
`
