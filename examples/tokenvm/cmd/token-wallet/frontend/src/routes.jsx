import App from "./App";

import Explorer from "./components/Explorer";
import Faucet from "./components/Faucet";
import Mint from "./components/Mint";
import Transfer from "./components/Transfer";
import Trade from "./components/Trade";
import Feed from "./components/Feed";

const routes = [
  {
    path: "/",
    element: <App />,
    children: [
      { index: true, element: <Explorer /> },
      {
        path: "explorer",
        element: <Explorer />,
      },
      {
        path: "faucet",
        element: <Faucet />,
      },
      {
        path: "mint",
        element: <Mint />,
      },
      {
        path: "transfer",
        element: <Transfer />,
      },
      {
        path: "trade",
        element: <Trade />,
      },
      {
        path: "feed",
        element: <Feed />,
      },
    ],
  },
];

export default routes;
