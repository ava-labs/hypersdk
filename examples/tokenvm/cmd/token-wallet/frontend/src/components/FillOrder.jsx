import { useRef } from "react";
import { App, InputNumber, Button } from "antd";
import { FillOrder as FOrder } from "../../wailsjs/go/main/App";

const FillOrder = ({ order }) => {
  const { message } = App.useApp();
  const amount = useRef("");
  const key = "updatable";

  const valueUpdate = (event) => {
    amount.current = event;
  };

  const fillOrder = () => {
    message.open({
      key,
      type: "loading",
      content: "Processing Transaction...",
      duration: 0,
    });
    (async () => {
      try {
        const start = new Date().getTime();
        console.log(order, amount.current);
        await FOrder(
          order.ID,
          order.Owner,
          order.InID,
          order.InputStep,
          order.OutID,
          amount.current
        );
        const finish = new Date().getTime();
        amount.current = "";
        message.open({
          key,
          type: "success",
          content: `Transaction Finalized (${finish - start} ms)`,
        });
      } catch (e) {
        message.open({
          key,
          type: "error",
          content: e.toString(),
        });
      }
    })();
  };

  return (
    <>
      <InputNumber
        placeholder={`Amount (${order.InSymbol})`}
        stringMode="true"
        style={{ width: "80%", margin: "8px 4px 0 0" }}
        onChange={valueUpdate}
        value={amount.current}
      />
      <Button type="primary" onClick={fillOrder} disabled={!window.HasBalance}>
        Fill
      </Button>
    </>
  );
};
export default FillOrder;
