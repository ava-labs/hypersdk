import App from "./App";

import Explorer from "./components/Explorer";
import Mine from "./components/Mine";
import Mint from "./components/Mint";
import Transfer from "./components/Transfer";

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
        path: "mine",
        element: <Mine />,
      },
      {
        path: "mint",
        element: <Mint />,
      },
      {
        path: "transfer",
        element: <Transfer />,
      },
    ],
  },
];

export default routes;
