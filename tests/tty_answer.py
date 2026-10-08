"""Answer prompts through a real controlling terminal."""

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

# Each step waits for a prompt, then types an answer. An answer such as
# signal:SIGTERM sends that signal to the process instead.
binary, *args = sys.argv[1:]
separator = args.index("--")
command, answers = args[:separator], args[separator + 1 :]
steps = [(answers[i].encode(), answers[i + 1].encode().decode("unicode_escape").encode()) for i in range(0, len(answers), 2)]
env = os.environ.copy()
env["TERM"] = "xterm-256color"
env["NO_COLOR"] = "1"

pid, master = pty.fork()
if pid == 0:
    os.execve(binary, [binary, *command], env)

fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 160, 0, 0))
output = bytearray()
searched = 0
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
        # Answer in a real terminal's order: termenv stops reading at the
        # cursor report, so a late color reply would be read as keystrokes.
        if not background_replied and b"\x1b]11;?" in output:
            os.write(master, b"\x1b]11;rgb:0000/0000/0000\x1b\\")
            background_replied = True
        while output.count(b"\x1b[6n") > cursor_replies:
            os.write(master, b"\x1b[1;1R")
            cursor_replies += 1
        if steps and output.find(steps[0][0], searched) != -1:
            prompt, answer = steps.pop(0)
            searched = output.find(prompt, searched) + len(prompt)
            time.sleep(0.2)
            if answer.startswith(b"signal:"):
                os.kill(pid, getattr(signal, answer[len(b"signal:"):].decode()))
                continue
            for key in answer:
                os.write(master, bytes([key]))
                time.sleep(0.02)
        exited, code = os.waitpid(pid, os.WNOHANG)
        if exited:
            status = code
            break
    assert not steps, f"Prompt not reached: {steps[0][0]!r} in {output!r}"
    assert status is not None, f"Command hung: {output!r}"
    sys.stdout.write(output.decode(errors="replace"))
    sys.exit(os.waitstatus_to_exitcode(status))
finally:
    if status is None:
        os.killpg(pid, signal.SIGKILL)
        os.waitpid(pid, 0)
    os.close(master)
