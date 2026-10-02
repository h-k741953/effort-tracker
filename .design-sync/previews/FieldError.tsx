import { FieldError } from "web";

export const Default = () => <FieldError id="field-error-default">0〜24 の範囲で入力してください。</FieldError>;

export const LongMessage = () => (
  <FieldError id="field-error-long">
    締め済みの勤務月は編集できません。修正が必要な場合は、承認者に差し戻しを依頼してください。
  </FieldError>
);
