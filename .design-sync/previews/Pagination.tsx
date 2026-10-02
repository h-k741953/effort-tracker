import { useState } from "react";
import { Pagination } from "web";

export const Middle = () => {
  const [page, setPage] = useState(3);
  return <Pagination page={page} pageCount={8} onPageChange={setPage} />;
};

export const FirstPage = () => {
  const [page, setPage] = useState(1);
  return <Pagination page={page} pageCount={8} onPageChange={setPage} />;
};

export const SinglePage = () => {
  const [page, setPage] = useState(1);
  return <Pagination page={page} pageCount={1} onPageChange={setPage} />;
};
