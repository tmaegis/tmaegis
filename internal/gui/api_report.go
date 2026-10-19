// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"bytes"
	"net/http"
	"slices"
	"strings"

	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/report"
)

var contentTypes = map[string]string{
	"json": "application/json", "jsonl": "application/x-ndjson", "sarif": "application/sarif+json",
	"asff": "application/json", "csv": "text/csv; charset=utf-8", "md": "text/markdown; charset=utf-8", "html": "text/html; charset=utf-8",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "pdf": "application/pdf",
}

func (s *Server) handleReports(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"reports": s.store.reports()})
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	sr, err := s.store.report(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "no such report")
		return
	}
	writeJSON(w, http.StatusOK, sr)
}

// handleReportFile serves /api/report/<id>.<format>: the same renderers as `tmaegis scan -f`.
func (s *Server) handleReportFile(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	dot := strings.LastIndex(file, ".")
	if dot < 1 {
		writeErr(w, http.StatusNotFound, "not_found", "no such report")
		return
	}
	id, format := file[:dot], strings.ToLower(file[dot+1:])
	if !slices.Contains(report.Formats, format) {
		writeErr(w, http.StatusBadRequest, "unsupported_format", "unsupported report format")
		return
	}
	sr, err := s.store.report(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "no such report")
		return
	}
	lang := s.defaultLang()
	if q := r.URL.Query().Get("lang"); q != "" && slices.Contains(i18n.Tags, q) {
		lang = q
	}
	var buf bytes.Buffer
	s.renderMu.Lock() // report.WriteLang sets a process-wide language
	err = report.WriteLang(&buf, format, sr.Report, lang)
	s.renderMu.Unlock()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "render_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", contentTypes[format])
	if format != "html" || r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="tmaegis-`+id+`.`+format+`"`)
	}
	_, _ = w.Write(buf.Bytes())
}
