function __complete_nctl
    set -lx COMP_LINE (commandline -cp)
    test -z (commandline -ct)
    and set COMP_LINE "$COMP_LINE "
    nctl
end
complete -f -c nctl -a "(__complete_nctl)"
