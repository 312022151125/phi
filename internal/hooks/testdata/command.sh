#!/bin/sh
# Command hook: echo a submit payload.
cat >/dev/null
echo '{"submit":"from command hook"}'
exit 0
