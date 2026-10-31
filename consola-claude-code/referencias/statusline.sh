#!/usr/bin/env bash
# Barra de estado de Claude Code (blueprint consola-claude-code): a la izquierda el modelo; a la derecha, uno por línea y
# pegados al borde, el % de contexto usado y el uso del plan (5h y semana). Un campo que falta se omite.
# El modo de la IA no llega en el JSON de la consola: solo lo muestra la barra de QDD (consola_qdd=si).
input=$(cat)
model=$(jq -r '.model.display_name // empty' <<<"$input")
ctx=$(jq -r '.context_window.used_percentage // empty' <<<"$input")
# Ancho de la terminal de la sesión (la barra corre sin terminal en su salida); 0 si no se sabe.
cols=$( (stty size </dev/tty) 2>/dev/null | cut -d' ' -f2)
[[ "$cols" =~ ^[0-9]+$ ]] || cols=${COLUMNS:-0}
util=$((cols - 2)) # termina 2 columnas antes del borde
tramo() { # $1 rótulo, $2 porcentaje, $3 epoch o ISO de renovación, $4 formato de fecha
	[ -n "$2" ] || return 0
	local t dias=(x lun mar mié jue vie sáb dom) f when="$3"
	t="$1 $(printf '%.0f' "$2")%"
	[[ "$when" =~ ^[0-9]+$ ]] && when="@$when"
	f="${4//%a/${dias[$(date -d "$when" +%u 2>/dev/null || echo 0)]}}"
	[ -n "$3" ] && date -d "$when" >/dev/null 2>&1 && t+=" ↻ $(date -d "$when" "+$f")"
	printf '%s' "$t"
}
derecha() { # rellena para que la línea termine a 2 columnas del borde
	local n=${#1}
	((cols > 0 && n < util)) && printf '%*s' $((util - n)) ''
	printf '%s' "$1"
}
read -r p5 r5 p7 r7 < <(jq -r '[.rate_limits.five_hour.used_percentage, .rate_limits.five_hour.resets_at, .rate_limits.seven_day.used_percentage, .rate_limits.seven_day.resets_at] | map(. // "") | map(if . == "" then "-" else tostring end) | join(" ")' <<<"$input")
for v in p5 r5 p7 r7; do [ "${!v}" = "-" ] && printf -v "$v" ''; done
lista=()
[ -n "$ctx" ] && lista+=("contexto $(printf '%.0f' "$ctx")%")
t=$(tramo "5h" "$p5" "$r5" "%H:%M") && [ -n "$t" ] && lista+=("$t")
t=$(tramo "semana" "$p7" "$r7" "%a %H:%M") && [ -n "$t" ] && lista+=("$t")
# Primera línea: el modelo y, si cabe a su derecha, el contexto; sin ancho conocido, juntos con «·».
primera=""
if [ ${#lista[@]} -gt 0 ] && [ -n "$model" ] && ((cols <= 0)); then
	primera="$model · ${lista[0]}" && lista=("${lista[@]:1}")
fi
if [ -z "$primera" ] && [ ${#lista[@]} -gt 0 ] && [ -n "$model" ] && ((${#model} + 2 + ${#lista[0]} <= util)); then
	primera="$model$(printf '%*s' $((util - ${#model} - ${#lista[0]})) '')${lista[0]}" && lista=("${lista[@]:1}")
fi
[ -z "$primera" ] && primera="$model"
lineas=()
[ -n "$primera" ] && lineas+=("$primera")
for l in "${lista[@]}"; do lineas+=("$(derecha "$l")"); done
(IFS=$'\n'; printf '%s' "${lineas[*]}")
