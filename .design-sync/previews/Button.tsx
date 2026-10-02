import { Button } from "web";

export const Variants = () => (
  <div className="flex items-center gap-2">
    <Button variant="primary">締める</Button>
    <Button variant="secondary">取消</Button>
    <Button variant="danger">差し戻す</Button>
  </div>
);

export const Default = () => <Button>一覧へ戻る</Button>;

export const Disabled = () => (
  <div className="flex items-center gap-2">
    <Button variant="primary" disabled>
      送信中…
    </Button>
    <Button variant="secondary" disabled>
      取消
    </Button>
  </div>
);
