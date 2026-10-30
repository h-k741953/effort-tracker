"use client";

// docs/specs/web-screens-implementation-design.md M-8。締め・承認・差戻しの区画と確認ステップ。

import { useState } from "react";
import { Button } from "@/components/button";
import { Card } from "@/components/card";
import { ConfirmDialog } from "@/components/confirm-dialog";

export function CloseAction({
  yearMonth,
  busy,
  onConfirm,
}: {
  yearMonth: string;
  busy: boolean;
  onConfirm: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [year, month] = yearMonth.split("-");
  return (
    <Card title="勤務月を締める">
      <p className="mb-3 text-sm">入力が済んだら締めてください。締めると承認されるまで編集できません。</p>
      <Button variant="primary" onClick={() => setOpen(true)}>
        締める
      </Button>
      <ConfirmDialog
        open={open}
        title={`${Number(year)}年${Number(month)}月の勤務月を締めますか？`}
        description={
          <>
            <p>締めると稼働実績は編集できなくなり、ご自身で取り消すことはできません。</p>
            <p>下書きに戻す方法は、承認者による差戻しだけです。</p>
          </>
        }
        confirmLabel="締める"
        confirmVariant="primary"
        onConfirm={() => {
          setOpen(false);
          if (!busy) onConfirm();
        }}
        onCancel={() => setOpen(false)}
      />
    </Card>
  );
}

type Review = "approve" | "reject" | null;

export function ReviewActions({
  busy,
  onApprove,
  onReject,
}: {
  busy: boolean;
  onApprove: () => void;
  onReject: () => void;
}) {
  const [review, setReview] = useState<Review>(null);
  const close = () => setReview(null);
  return (
    <Card title="承認">
      <p className="mb-3 text-sm">内容を確認して、承認または差戻しをしてください。</p>
      <div className="flex gap-2">
        <Button variant="danger" onClick={() => setReview("reject")}>
          差戻し
        </Button>
        <Button variant="primary" onClick={() => setReview("approve")}>
          承認
        </Button>
      </div>
      <ConfirmDialog
        open={review === "approve"}
        title="この勤務月を承認しますか？"
        description="承認すると承認済になります。承認済の勤務月は、差戻しを含めて以降の操作ができません。"
        confirmLabel="承認する"
        confirmVariant="primary"
        onConfirm={() => {
          close();
          if (!busy) onApprove();
        }}
        onCancel={close}
      />
      <ConfirmDialog
        open={review === "reject"}
        title="この勤務月を差し戻しますか？"
        description="差し戻すと下書きに戻り、技術者が稼働実績を修正できるようになります。"
        confirmLabel="差し戻す"
        confirmVariant="danger"
        onConfirm={() => {
          close();
          if (!busy) onReject();
        }}
        onCancel={close}
      />
    </Card>
  );
}
