"""Exercise cancellation using a real controlling terminal, not piped stdin."""

import errno
import fcntl
import os
import pty
import select
import signal
import struct
import sys
import termios
import time

binary, stage, key = sys.argv[1:]
env = os.environ.copy()
env["TERM"] = "xterm-256color"
env["NO_COLOR"] = "1"
if stage == "path":
    env["GIT_WT_SELECT"] = "origin/feature"
    prompt = b"Enter worktree path"
else:
    env.pop("GIT_WT_SELECT", None)
    prompt = b"Select branch or create new"

pid, master = pty.fork()
if pid == 0:
    os.execve(binary, [binary, "add"], env)

fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 120, 0, 0))
output = bytearray()
sent = False
status = None
cursor_replies = 0
background_replied = False
deadline = time.monotonic() + 20
try:
    while time.monotonic() < deadline:
        if select.select([master], [], [], 0.05)[0]:
            try:
                output.extend(os.read(master, 65536))
            except OSError as error:
                if error.errno != errno.EIO:
                    raise
        while output.count(b"\x1b[6n") > cursor_replies:
            os.write(master, b"\x1b[1;1R")
            cursor_replies += 1
        if not background_replied and b"\x1b]11;?" in output:
            os.write(master, b"\x1b]11;rgb:0000/0000/0000\x1b\\")
            background_replied = True
        if not sent and prompt in output:
            os.write(master, b"\x03" if key == "ctrl-c" else b"\x1b")
            sent = True
        exited, code = os.waitpid(pid, os.WNOHANG)
        if exited:
            status = code
            break
    assert sent, f"Prompt not reached: {output!r}"
    assert status is not None, f"Cancellation hung: {output!r}"
    assert not os.path.exists("feature"), f"Cancellation created a worktree: {output!r}"
    if stage == "path":
        assert os.waitstatus_to_exitcode(status) != 0, output
    else:
        assert os.waitstatus_to_exitcode(status) == 0, output
finally:
    if status is None:
        os.killpg(pid, signal.SIGKILL)
        os.waitpid(pid, 0)
    os.close(master)
