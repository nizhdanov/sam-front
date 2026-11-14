package http

import (
	"net/http"

	"sam/pkg/auth"
	"sam/pkg/httpx"
)

var suggestions = map[auth.Role][]string{
	auth.RoleAdmin: {
		"У кого стоит нелицензионный Adobe Photoshop?",
		"Сколько лицензий простаивает?",
		"Где у нас дефицит лицензий?",
		"На каких устройствах установлено запрещённое ПО?",
		"Какие договоры истекают в ближайший месяц?",
		"Сколько ПО не распознано и ждёт сопоставления?",
	},
	auth.RoleUser: {
		"Какой бесплатный архиватор можно установить?",
		"Какую версию 7-Zip использовать?",
		"Можно ли установить uTorrent?",
		"Есть ли на моём компьютере запрещённые программы?",
		"Какие программы на моём компьютере устарели?",
	},
}

func (h *handler) suggestions(w http.ResponseWriter, r *http.Request) {
	items := suggestions[auth.ClaimsFrom(r.Context()).Role]
	if items == nil {
		items = []string{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}
