until go run cahbot.go; do
    echo "cahbot crashed with exit code $?.  Respawning.." >&2
    sleep 1
done
