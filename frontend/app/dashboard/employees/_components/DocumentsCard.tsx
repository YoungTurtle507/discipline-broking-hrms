"use client";

import { useEffect, useRef, useState } from "react";
import { CircleNotch, FileText, UploadSimple } from "@phosphor-icons/react";
import { apiFetch, apiUpload, getBackendUrl, ApiError } from "@/lib/api";
import { formatDate } from "../../_components/my/types";

export type DocumentType = "OFFER_LETTER" | "AADHAR_CARD" | "PAN_CARD" | "MARKSHEET";

export const DOCUMENT_SLOTS: { type: DocumentType; label: string }[] = [
  { type: "OFFER_LETTER", label: "Offer Letter" },
  { type: "AADHAR_CARD", label: "Aadhar Card" },
  { type: "PAN_CARD", label: "PAN Card" },
  { type: "MARKSHEET", label: "10th / 12th Marksheet" },
];

type DocumentInfo = { document_type: DocumentType; uploaded_at: string };

// The employee profile's document slots — Offer Letter, Aadhar Card, and a
// single Marksheet slot (10th or 12th, whichever the employee has). Mirrors
// the leave document upload/download pattern, just per employee instead of
// per leave request, with three fixed slots instead of one.
export function DocumentsCard({ employeeId }: { employeeId: number }) {
  const [docs, setDocs] = useState<Record<string, DocumentInfo> | null>(null);
  const [uploadingType, setUploadingType] = useState<DocumentType | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const pendingType = useRef<DocumentType | null>(null);

  function load() {
    apiFetch<{ documents: Record<string, DocumentInfo> | null }>(`/api/v1/employees/${employeeId}/documents`)
      .then((d) => setDocs(d.documents ?? {}))
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load documents"));
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [employeeId]);

  function handleUploadClick(type: DocumentType) {
    pendingType.current = type;
    fileInputRef.current?.click();
  }

  async function handleFileSelected(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    const type = pendingType.current;
    if (!file || !type) return;

    setError(null);
    setUploadingType(type);
    try {
      const formData = new FormData();
      formData.append("file", file);
      await apiUpload(`/api/v1/employees/${employeeId}/documents/${type}`, formData);
      load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to upload the document");
    } finally {
      setUploadingType(null);
    }
  }

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-card">
      <h2 className="mb-4 text-base font-semibold text-slate-900">Documents</h2>
      <input ref={fileInputRef} type="file" accept=".pdf,.jpg,.jpeg,.png,.webp" onChange={handleFileSelected} className="hidden" />
      {error && <p className="mb-3 text-sm text-error">{error}</p>}
      <div className="flex flex-col divide-y divide-slate-100">
        {DOCUMENT_SLOTS.map(({ type, label }) => {
          const info = docs?.[type];
          const uploading = uploadingType === type;
          return (
            <div key={type} className="flex items-center justify-between gap-3 py-3 first:pt-0 last:pb-0">
              <div className="flex min-w-0 items-center gap-2.5">
                <FileText size={18} className={info ? "text-brand-blue" : "text-slate-300"} />
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-slate-800">{label}</p>
                  <p className="truncate text-xs text-slate-400">
                    {info ? `On file · ${formatDate(info.uploaded_at)}` : "Not uploaded"}
                  </p>
                </div>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                {info && (
                  <a
                    href={`${getBackendUrl()}/api/v1/employees/${employeeId}/documents/${type}`}
                    target="_blank"
                    rel="noreferrer"
                    className="text-xs font-semibold text-slate-500 hover:text-brand-blue hover:underline"
                  >
                    View
                  </a>
                )}
                <button
                  type="button"
                  onClick={() => handleUploadClick(type)}
                  disabled={uploading}
                  className="flex items-center gap-1.5 rounded-lg border border-slate-200 px-2.5 py-1.5 text-xs font-semibold text-slate-600 hover:bg-slate-50 disabled:opacity-60"
                >
                  {uploading ? <CircleNotch size={13} className="animate-spin" /> : <UploadSimple size={13} weight="bold" />}
                  {info ? "Replace" : "Upload"}
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
