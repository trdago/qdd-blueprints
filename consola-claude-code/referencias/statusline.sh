#!/usr/bin/env bash
# Barra de estado de Claude Code: proyecto · modelo · % de contexto usado · uso del plan (5h y semana) · estado por frente
input=$(cat)
proj=$(jq -r '.workspace.project_dir // .workspace.current_dir // .cwd // empty' <<<"$input")
model=$(jq -r '.model.display_name // empty' <<<"$input")
ctx=$(jq -r '.context_window.used_percentage // empty' <<<"$input")
line="📁 ${proj##*/}"
[ -n "$model" ] && line+=" · $model"
[ -n "$ctx" ] && line+=" · contexto $(printf '%.0f' "$ctx")%"
# Uso del plan: ventana de 5 h (corto plazo) y de 7 días. Se omite el tramo si falta el campo.
tramo() { # $1 rótulo, $2 porcentaje, $3 epoch de renovación, $4 formato de fecha
	[ -n "$2" ] || return 0
	local t
	t="$1 $(printf '%.0f' "$2")%"
	local dias=(x lun mar mié jue vie sáb dom) f when="$3"
	[[ "$when" =~ ^[0-9]+$ ]] && when="@$when" # epoch en segundos o fecha ISO
	f="${4//%a/${dias[$(date -d "$when" +%u 2>/dev/null || echo 0)]}}"
	[ -n "$3" ] && date -d "$when" >/dev/null 2>&1 && t+=" (↻ $(date -d "$when" "+$f"))"
	printf ' · %s' "$t"
}
read -r p5 r5 p7 r7 < <(jq -r '[.rate_limits.five_hour.used_percentage, .rate_limits.five_hour.resets_at, .rate_limits.seven_day.used_percentage, .rate_limits.seven_day.resets_at] | map(. // "") | map(if . == "" then "-" else tostring end) | join(" ")' <<<"$input")
for v in p5 r5 p7 r7; do [ "${!v}" = "-" ] && printf -v "$v" ''; done
line+="$(tramo "5h" "$p5" "$r5" "%H:%M")"
line+="$(tramo "semana" "$p7" "$r7" "%a %H:%M")"
# Segmento de estado (si qdd está instalado): resumen local del estado por frente del proyecto,
# con tope de 100 ms. Sin archivo, sin qdd o con un qdd antiguo no imprime nada.
if [ -n "$proj" ] && command -v qdd >/dev/null 2>&1; then
	est=$(timeout 0.1 qdd estado statusline --dir "$proj" 2>/dev/null)
	[ -n "$est" ] && line+=" · $est"
fi
printf '%s' "$line"
