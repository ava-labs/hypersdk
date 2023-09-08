import App from "./App";

import Explorer from "./components/Explorer";
import Mint from "./components/Mint";

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
        path: "mint",
        element: <Mint />,
      },
    ],
  },
];

export default routes;
